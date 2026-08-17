import type { FlagKind } from '../../data/overview'
import type { IconName } from '../../icons/icons'
import { tokens } from '../../tokens/tokens'
import { FLAG_INK } from './flag-palette'

/**
 * What each flag is called and how it reads. The prototype's FLAG_META, ported — a vocabulary rather
 * than a computation, so it stays on this side of the seam.
 */
export const FLAGS: Record<FlagKind, { label: string; icon: IconName; fg: string; bg: string }> = {
  missed: { label: 'Пропуск дозы', icon: 'exclamation-circle', fg: FLAG_INK.danger, bg: tokens.dangerBg },
  side: { label: 'Побочный эффект', icon: 'heart', fg: FLAG_INK.warning, bg: tokens.warningBg },
  biomarker: { label: 'Биомаркер вне нормы', icon: 'arrow-trending-up', fg: FLAG_INK.warning, bg: tokens.warningBg },
  message: { label: 'Ждёт ответа', icon: 'chat-bubble', fg: FLAG_INK.info, bg: tokens.infoBg },
  titration: { label: 'Пора титровать', icon: 'beaker', fg: tokens.forest800, bg: tokens.forest50 },
  cycle: { label: 'Цикл завершается', icon: 'clock', fg: tokens.sand900, bg: tokens.sand100 },
}

export const STATUS_LABEL = {
  attention: 'Внимание',
  watch: 'Наблюдение',
  track: 'В норме',
} as const
