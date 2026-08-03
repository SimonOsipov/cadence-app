package app.cadence.screens.today

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import app.cadence.design.Cadence
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.format.greeting
import app.cadence.shared.repository.TodaySummary

/**
 * «Сегодня», ported from mobile/src/features/today/TodayScreen.tsx.
 *
 * Takes a [TodaySummary] and lambdas, never a repository: the screen is handed
 * a day and reports taps, which is what lets the mock be swapped for the Ktor
 * client without this file changing. Everything it renders that carries a
 * number goes through `app.cadence.format`.
 */
@Composable
fun TodayScreen(
    summary: TodaySummary,
    patientName: String,
    modifier: Modifier = Modifier,
    onLogDose: () -> Unit = { },
    onOpenQuickFeel: () -> Unit = { },
    onOpenJournal: () -> Unit = { },
    onOpenVials: () -> Unit = { },
    onOpenTrends: () -> Unit = { },
    onOpenChat: () -> Unit = { },
    onOpenSchedule: () -> Unit = { },
    onOpenLearn: () -> Unit = { },
    onOpenProfile: () -> Unit = { },
    onLogMeal: () -> Unit = { },
    onOpenRecipes: () -> Unit = { },
    onOpenNutrition: () -> Unit = { },
) {
    Column(modifier.fillMaxSize()) {
        TodayHeader(
            patientName = patientName,
            greeting = greeting(summary.date, summary.partOfDay, summary.cycleWeek),
            onOpenChat = onOpenChat,
            onOpenSchedule = onOpenSchedule,
            onOpenLearn = onOpenLearn,
            onOpenProfile = onOpenProfile,
        )

        Column(
            modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        ) {
            TodayHero(
                compoundName = summary.nextDoseCompound?.nameRu,
                dose = summary.nextDose?.dose,
                logged = summary.doseLoggedToday,
                onLogDose = onLogDose,
                onOpenQuickFeel = onOpenQuickFeel,
            )

            BiomarkerGlance(
                series = summary.weightSeries,
                latest = summary.weightKg,
                onOpen = onOpenTrends,
            )

            WellbeingNudge(onOpen = onOpenJournal)

            ProtocolStrip(rows = summary.weekProtocol, onOpenSchedule = onOpenSchedule)

            MealHero(
                eaten = summary.mealMacros,
                targets = summary.targets,
                onLogMeal = onLogMeal,
                onOpenRecipes = onOpenRecipes,
            )

            TodayMeals(
                eaten = summary.mealMacros,
                targets = summary.targets,
                onOpenNutrition = onOpenNutrition,
            )

            summary.reorder?.let { hint ->
                ReorderCard(
                    compoundName =
                        summary.weekProtocol
                            .firstOrNull { it.compound?.id == hint.compoundId }
                            ?.compound
                            ?.nameRu,
                    weeksLeft = hint.weeksLeft,
                    onOpenVials = onOpenVials,
                )
            }
        }
    }
}

/** The fixed strip above the scroll region: greeting, name, four ways out. */
@Composable
private fun TodayHeader(
    patientName: String,
    greeting: String,
    onOpenChat: () -> Unit,
    onOpenSchedule: () -> Unit,
    onOpenLearn: () -> Unit,
    onOpenProfile: () -> Unit,
) {
    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(horizontal = CadenceSpacing.xl, vertical = CadenceSpacing.md),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceMeta(greeting)

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CadenceTitle(patientName, maxLines = 1)

            Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                CadenceIconButton(CadenceIcons.chatBubble, "Чат", onOpenChat, tint = Cadence.palette.forestPillFg)
                CadenceIconButton(CadenceIcons.calendar, "Расписание", onOpenSchedule)
                CadenceIconButton(CadenceIcons.bookOpen, "Обучение", onOpenLearn)
                CadenceIconButton(CadenceIcons.user, "Профиль", onOpenProfile)
            }
        }
    }
}
