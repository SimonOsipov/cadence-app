package app.cadence.shared.domain

import kotlin.time.Instant

/** §03's article categories. */
enum class ArticleCategory(
    val code: String,
) {
    PEPTIDES("peptides"),
    PROTOCOLS("protocols"),
    NUTRITION("nutrition"),
    TECHNIQUE("technique"),
    SIDES("sides"),
    RECOVERY("recovery"),
}

/** §03: `blocks jsonb p|h|quote|key`. */
enum class ArticleBlockKind(
    val code: String,
) {
    PARAGRAPH("p"),
    HEADING("h"),
    QUOTE("quote"),
    KEY("key"),
}

data class ArticleBlock(
    val kind: ArticleBlockKind,
    val text: String,
)

/** §03's `articles` — seeded via data migration; authoring UI is out of scope. */
data class Article(
    val id: ArticleId,
    val slug: String,
    val category: ArticleCategory,
    val eyebrow: String?,
    val title: String,
    val dek: String?,
    val readMin: Int,
    val featured: Boolean,
    val personalized: Boolean,
    val blocks: List<ArticleBlock>,
    val publishedAt: Instant?,
)
