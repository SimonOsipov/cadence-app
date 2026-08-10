package app.cadence.design

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.generated.Res
import org.jetbrains.compose.resources.ExperimentalResourceApi
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * What the bundled font *files* contain.
 *
 * Everything else about the typography is asserted through the scale — which
 * face is bound to which role, which weights a family offers. None of that
 * reads a byte of the fonts, so all of it stays green if a file is replaced by
 * a Latin-only subset. That is not a hypothetical: Google Fonts serves subsets
 * on request, and "re-download the font" is an ordinary step when a design
 * updates.
 *
 * Since the entire reason the body face changed from DM Sans to Golos Text is
 * Cyrillic coverage, this is the test of the claim rather than of its
 * neighbour.
 */
@OptIn(ExperimentalTestApi::class, ExperimentalResourceApi::class)
class BundledFontTest {
    @Test
    fun everyBundledFaceCoversTheCyrillicAlphabet() =
        runComposeUiTest {
            listOf(
                "GolosText.ttf",
                "CormorantGaramond.ttf",
                "CormorantGaramondItalic.ttf",
                "JetBrainsMono.ttf",
            ).forEach { file ->
                val bytes = readFontBytes(file)
                val covered = cmapCodePoints(bytes)

                // А–я, plus Ё and ё — which live outside the contiguous block
                // and are the pair a careless subset drops first.
                val required = (0x0410..0x044F).toList() + listOf(0x0401, 0x0451)
                val missing = required.filterNot { it in covered }

                assertTrue(
                    missing.isEmpty(),
                    "$file does not cover ${missing.map { it.toChar() }} — " +
                        "Russian copy would render in a system fallback",
                )
            }
        }

    private fun androidx.compose.ui.test.ComposeUiTest.readFontBytes(file: String): ByteArray {
        var bytes: ByteArray? = null
        setContent {
            LaunchedEffect(file) {
                bytes = Res.readBytes("font/$file")
            }
        }
        waitUntil("$file was never read") { bytes != null }
        return bytes!!
    }
}

/**
 * The code points a font's `cmap` maps, for the subtable formats these files
 * use — 4 (BMP) and 12 (full range).
 *
 * Deliberately not a general parser: it reads the two formats present in the
 * bundled files and ignores the rest, because its only job is to answer whether
 * a glyph exists.
 */
internal fun cmapCodePoints(font: ByteArray): Set<Int> {
    val reader = BigEndian(font)
    val cmap = reader.tableOffset("cmap")
    require(cmap >= 0) { "font has no cmap table" }

    val covered = mutableSetOf<Int>()
    val subtableCount = reader.u16(cmap + 2)
    for (i in 0 until subtableCount) {
        val offset = cmap + reader.u32(cmap + 4 + i * 8 + 4)
        when (reader.u16(offset)) {
            4 -> covered += reader.format4Ranges(offset)
            12 -> covered += reader.format12Ranges(offset)
        }
    }
    return covered
}

/** Big-endian reads over the font's bytes, plus the two subtable shapes. */
private class BigEndian(
    private val bytes: ByteArray,
) {
    fun u8(at: Int) = bytes[at].toInt() and 0xFF

    fun u16(at: Int) = (u8(at) shl 8) or u8(at + 1)

    fun u32(at: Int) = (u16(at).toLong() shl 16 or u16(at + 2).toLong()).toInt()

    fun tableOffset(name: String): Int {
        for (i in 0 until u16(4)) {
            val record = 12 + i * 16
            // Four ASCII bytes, read by hand: String(bytes, charset) is a JVM
            // overload and this is common code.
            val tag = (0 until 4).map { u8(record + it).toChar() }.joinToString("")
            if (tag == name) return u32(record + 8)
        }
        return -1
    }

    /** Segment-mapped BMP coverage. */
    fun format4Ranges(offset: Int): List<Int> {
        val segCount = u16(offset + 6) / 2
        val endsAt = offset + 14
        val startsAt = endsAt + segCount * 2 + 2
        return (0 until segCount).flatMap { s ->
            val start = u16(startsAt + s * 2)
            val end = u16(endsAt + s * 2)
            // 0xFFFF terminates the segment list; it is not coverage.
            if (start <= end && start != 0xFFFF) (start..end).toList() else emptyList()
        }
    }

    /** Grouped coverage across the full range. */
    fun format12Ranges(offset: Int): List<Int> =
        (0 until u32(offset + 12)).flatMap { g ->
            val at = offset + 16 + g * 12
            (u32(at)..u32(at + 4)).toList()
        }
}
