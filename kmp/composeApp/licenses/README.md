# Bundled typeface licences

The three families in `src/commonMain/composeResources/font/` are distributed
under the SIL Open Font License 1.1. The OFL requires its text and the
copyright notice to travel with the fonts, so they live here rather than being
claimed in a code comment.

| Family | Files | Licence |
|---|---|---|
| Cormorant Garamond | `CormorantGaramond.ttf`, `CormorantGaramondItalic.ttf` | [OFL-CormorantGaramond.txt](OFL-CormorantGaramond.txt) |
| Golos Text | `GolosText.ttf` | [OFL-GolosText.txt](OFL-GolosText.txt) |
| JetBrains Mono | `JetBrainsMono.ttf` | [OFL-JetBrainsMono.txt](OFL-JetBrainsMono.txt) |

They sit outside `composeResources/` on purpose: that directory is scanned by
the Compose resource generator, which would turn a licence file into a
resource accessor.

Reserved Font Names apply — the OFL forbids distributing a modified version
under the original name. We ship these unmodified.
