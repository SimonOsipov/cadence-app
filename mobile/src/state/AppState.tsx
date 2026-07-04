// App-level state — ports the useState groups from prototype.jsx PrototypeApp.
// Everything is in-memory local state; no backend, single fake user.
import React, { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import {
  DAY_STATES,
  dayTotals,
  suggestNextMeal,
  pickMealCoach,
  MEAL_TARGETS,
} from '../features/meal/data';
import { VIAL_INVENTORY, inventorySummary } from '../features/inventory/data';
import { JOURNAL } from '../features/journal/data';
import { BODY } from '../features/body/data';
import { RECIPES } from '../features/recipe/data';

// Coach lines — from shared.jsx.
export const COACH_LINES = [
  { lead: 'Вы стали', emph: 'легче', tail: 'и спите глубже.', note: 'Ровно там, где мы и надеялись. Держите утренний ритм.' },
  { lead: 'Восемь дней стабильного сна —', emph: 'HRV растёт.', tail: '', note: '+3 мс за неделю. Восстановление догоняет.' },
  { lead: 'Половина четвёртой недели.', emph: 'Держите ритм.', tail: '', note: 'Следующая доза — воскресенье. Та же ротация, то же утро.' },
  { lead: 'Хотите среду помягче?', emph: 'Вы заслужили тихий день.', tail: '', note: 'Перенесите BPC-157 на вечер, силовую — пропустите.' },
];

const DAY_STATE = 'midday';

type ConfirmSheet = { kcal: number; mealName: string } | null;

export type AppState = {
  // dosing
  doseLogged: boolean;
  setDoseLogged: (v: boolean) => void;
  coachIndex: number;
  coachLine: (typeof COACH_LINES)[number];
  completeDoseLog: (site?: string) => void;
  lastDoseSite: string | null;
  // trends (shared between landing and detail, like the prototype's app-level tweaks)
  timeframe: string;
  setTimeframe: (v: string) => void;
  activeBiomarker: string;
  setActiveBiomarker: (v: string) => void;
  // meals
  meals: any[];
  mealTotals: any;
  mealTargets: typeof MEAL_TARGETS;
  mealCoach: any;
  mealHeroSuggestion: any;
  logMeal: (meal: any) => void;
  addRecipeToDay: (recipe: any, portions: number) => void;
  confirmSheet: ConfirmSheet;
  // inventory
  vials: any[];
  invSummary: any;
  reorderHint: any;
  sealedOpen: boolean;
  setSealedOpen: (v: boolean) => void;
  // journal
  journalEntries: any[];
  saveQuickFeel: (entry: any) => void;
  // body
  bodyState: any;
  addBodyMeasure: (id: string, val: number | string) => void;
  addBodyPhoto: () => void;
  // recipes
  userRecipes: any[];
  allRecipes: any[];
  saveRecipe: (recipe: any) => void;
};

const Ctx = createContext<AppState | null>(null);

export function AppStateProvider({ children }: { children: React.ReactNode }) {
  const [doseLogged, setDoseLogged] = useState(false);
  const [coachIndex, setCoachIndex] = useState(0);
  const [lastDoseSite, setLastDoseSite] = useState<string | null>(null);
  const [timeframe, setTimeframe] = useState('3m');
  const [activeBiomarker, setActiveBiomarker] = useState('weight');

  const [userMeals, setUserMeals] = useState<any[]>([]);
  const [mealCoach, setMealCoach] = useState<any>(null);
  const [confirmSheet, setConfirmSheet] = useState<ConfirmSheet>(null);
  const confirmTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [sealedOpen, setSealedOpen] = useState(false);
  const [journalEntries, setJournalEntries] = useState<any[]>(JOURNAL.ENTRIES);
  const [bodyState, setBodyState] = useState<any>(() => BODY.seed());
  const [userRecipes, setUserRecipes] = useState<any[]>([]);

  const baseMeals = DAY_STATES[DAY_STATE].meals;
  const meals = useMemo(() => [...baseMeals, ...userMeals], [baseMeals, userMeals]);
  const mealTotals = useMemo(() => dayTotals(meals), [meals]);
  const mealHeroSuggestion = useMemo(
    () => ({ ...suggestNextMeal(meals, DAY_STATES[DAY_STATE].now), now: DAY_STATES[DAY_STATE].now }),
    [meals],
  );

  const showConfirm = useCallback((sheet: ConfirmSheet) => {
    if (confirmTimer.current) clearTimeout(confirmTimer.current);
    setConfirmSheet(sheet);
    confirmTimer.current = setTimeout(() => setConfirmSheet(null), 1700);
  }, []);

  const completeDoseLog = useCallback((site?: string) => {
    setDoseLogged(true);
    if (site) setLastDoseSite(site);
    setCoachIndex((i) => (i + 1) % COACH_LINES.length);
  }, []);

  const appendMeal = useCallback(
    (newMeal: any) => {
      const nextMeals = [...meals, newMeal];
      const nextTotals = dayTotals(nextMeals);
      setUserMeals((prev) => [...prev, newMeal]);
      setMealCoach(pickMealCoach({ meals: nextMeals, lastMeal: newMeal, totals: nextTotals }));
      showConfirm({ kcal: nextTotals.kcal, mealName: newMeal.mealName });
    },
    [meals, showConfirm],
  );

  const logMeal = useCallback(
    (meal: any) => {
      appendMeal({
        id: `u-${Date.now()}`,
        time: meal.time,
        mealName: meal.mealName,
        items: meal.items,
        totals: meal.totals,
      });
    },
    [appendMeal],
  );

  const addRecipeToDay = useCallback(
    (recipe: any, portions: number) => {
      const meal = RECIPES.toMeal(recipe, portions, DAY_STATES[DAY_STATE].now);
      appendMeal({ id: `u-${Date.now()}`, ...meal });
    },
    [appendMeal],
  );

  const saveQuickFeel = useCallback((entry: any) => {
    setJournalEntries((prev) => {
      const rest = prev.filter((e) => e.day !== JOURNAL.TODAY_DAY);
      return [...rest, { id: `u-${Date.now()}`, day: JOURNAL.TODAY_DAY, ...entry }];
    });
  }, []);

  const addBodyMeasure = useCallback((id: string, val: number | string) => {
    setBodyState((prev: any) => ({
      ...prev,
      hist: { ...prev.hist, [id]: [...(prev.hist[id] ?? []), Number(val)] },
    }));
  }, []);

  const addBodyPhoto = useCallback(() => {
    setBodyState((prev: any) => {
      const w = prev.hist.weight[prev.hist.weight.length - 1];
      const week = `Нед ${JOURNAL.weekOf(JOURNAL.TODAY_DAY)}`;
      return { ...prev, photos: [...prev.photos, { id: `u-${Date.now()}`, week, weight: w }] };
    });
  }, []);

  const saveRecipe = useCallback((recipe: any) => {
    setUserRecipes((prev) => [recipe, ...prev]);
  }, []);

  const allRecipes = useMemo(() => [...userRecipes, ...RECIPES.STARTERS], [userRecipes]);

  const invSummary = useMemo(() => inventorySummary(VIAL_INVENTORY), []);
  const reorderHint = invSummary.reorder[0] || null;

  const value: AppState = {
    doseLogged,
    setDoseLogged,
    coachIndex,
    coachLine: COACH_LINES[coachIndex % COACH_LINES.length],
    completeDoseLog,
    lastDoseSite,
    timeframe,
    setTimeframe,
    activeBiomarker,
    setActiveBiomarker,
    meals,
    mealTotals,
    mealTargets: MEAL_TARGETS,
    mealCoach,
    mealHeroSuggestion,
    logMeal,
    addRecipeToDay,
    confirmSheet,
    vials: VIAL_INVENTORY,
    invSummary,
    reorderHint,
    sealedOpen,
    setSealedOpen,
    journalEntries,
    saveQuickFeel,
    bodyState,
    addBodyMeasure,
    addBodyPhoto,
    userRecipes,
    allRecipes,
    saveRecipe,
  };

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAppState(): AppState {
  const v = useContext(Ctx);
  if (!v) throw new Error('useAppState outside AppStateProvider');
  return v;
}
