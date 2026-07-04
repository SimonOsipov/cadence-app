// Cadence — interactive prototype
// Single device frame (iOS or Android). Today screen is the home; sliding
// overlays host the Log Dose wizard, Trends, Trend Detail, Log Meal, and
// Nutrition Today.

const PROTO_TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{}/*EDITMODE-END*/;

function PrototypeApp() {
  const [t, setTweak] = useTweaks(PROTO_TWEAK_DEFAULTS);
  const platform = 'ios';
  const headerVariant = 'cream';
  const showAnnotations = true;
  const biomarker = 'weight';
  const timeframe = '3m';
  const dayState = 'midday';
  const mealMode = 'chat';

  // ── App-level state ─────────────────────────────────────────
  const [doseLogged, setDoseLogged] = React.useState(false);
  const [screen, setScreen] = React.useState('today');   // 'today' | 'log-dose' | 'trends' | 'trend-detail' | 'log-meal' | 'nutrition'
  const [prevScreen, setPrevScreen] = React.useState('today');
  const [logSession, setLogSession] = React.useState(0);
  const [mealSession, setMealSession] = React.useState(0);
  const [coachIndex, setCoachIndex] = React.useState(0);

  // Meal state. baseMeals comes from the day-state preset; userMeals is what
  // the user has logged through the flow. Reset baseMeals on day-state change.
  const [userMeals, setUserMeals] = React.useState([]);
  const [mealCoach, setMealCoach] = React.useState(null);
  const [confirmSheet, setConfirmSheet] = React.useState(null);
  const [actionSheetOpen, setActionSheetOpen] = React.useState(false);

  // Inventory state
  const [vialDetailId, setVialDetailId] = React.useState(null);
  const [sealedOpen, setSealedOpen]     = React.useState(false);
  const [addVialSession, setAddVialSession] = React.useState(0);

  // Chat state
  const [chatThreadId, setChatThreadId] = React.useState('ksenia');

  // Reset baseMeals → derived from dayState
  const baseMeals = DAY_STATES[dayState].meals;
  const meals = [...baseMeals, ...userMeals];
  const mealTotals = dayTotals(meals);
  const mealHeroSuggestion = {
    ...suggestNextMeal(meals, DAY_STATES[dayState].now),
    now: DAY_STATES[dayState].now,
  };

  // Whenever the day-state tweak changes, also clear user-added meals so
  // the preset reads cleanly. Otherwise switching tweaks layers oddly.
  React.useEffect(() => {
    setUserMeals([]);
    setMealCoach(null);
  }, [dayState]);

  // ── Navigation ──────────────────────────────────────────────
  const openActionSheet = () => setActionSheetOpen(true);
  const closeActionSheet = () => setActionSheetOpen(false);

  const openLog = () => { setLogSession(s => s + 1); setScreen('log-dose'); };
  const closeLog = () => setScreen(prevScreen === 'log-dose' ? 'today' : prevScreen);
  const completeLog = () => {
    setDoseLogged(true);
    setCoachIndex(i => (i + 1) % COACH_LINES.length);
    setScreen(prevScreen === 'log-dose' ? 'today' : prevScreen);
  };

  const openTrends = () => { setPrevScreen('today'); setScreen('trends'); };
  const openTrend = (biomarkerId) => {
    if (biomarkerId && biomarkerId !== biomarker) setTweak('biomarker', biomarkerId);
    setPrevScreen(screen === 'today' ? 'today' : 'trends');
    setScreen('trend-detail');
  };

  const openMealLog = () => {
    setMealSession(s => s + 1);
    setPrevScreen(screen === 'nutrition' ? 'nutrition' : 'today');
    setScreen('log-meal');
  };
  const closeMealLog = () => setScreen(prevScreen === 'nutrition' ? 'nutrition' : 'today');
  const completeMealLog = (meal) => {
    // Add to user meals
    const newMeal = {
      id: `u-${Date.now()}`,
      time: meal.time,
      mealName: meal.mealName,
      items: meal.items,
      totals: meal.totals,
    };
    const nextMeals = [...meals, newMeal];
    const nextTotals = dayTotals(nextMeals);
    setUserMeals(prev => [...prev, newMeal]);
    // Coach line picks based on running totals
    setMealCoach(pickMealCoach({ meals: nextMeals, lastMeal: newMeal, totals: nextTotals }));
    // Show confirm sheet briefly, then close to Today
    setConfirmSheet({ kcal: nextTotals.kcal, mealName: meal.mealName });
    setTimeout(() => {
      setConfirmSheet(null);
      setScreen(prevScreen === 'nutrition' ? 'nutrition' : 'today');
    }, 1700);
  };

  const openNutrition = () => { setPrevScreen('today'); setScreen('nutrition'); };

  // Inventory navigation
  const openVials       = () => { setPrevScreen('today'); setScreen('vials'); };
  const openAddVial     = () => { setAddVialSession(s => s + 1); setScreen('add-vial'); };
  const closeAddVial    = () => setScreen('vials');
  const completeAddVial = () => setScreen('vials');

  // Chat navigation
  const openChat       = () => { setChatThreadId('ksenia'); setPrevScreen('today'); setScreen('chat-thread'); };
  const openChatList   = () => { setScreen('chat-list'); };
  const openChatThread = (id) => { setChatThreadId(id); setScreen('chat-thread'); };

  // Profile navigation
  const openProfile    = () => { setPrevScreen('today'); setScreen('profile'); };
  const openChatFromProfile = (id) => { setChatThreadId(id || 'ksenia'); setPrevScreen('profile'); setScreen('chat-thread'); };

  // Schedule navigation
  const openSchedule   = () => { setPrevScreen('today'); setScreen('schedule'); };
  const openLogFromSchedule = () => { setPrevScreen('schedule'); setLogSession(s => s + 1); setScreen('log-dose'); };

  // Learn navigation
  const [articleId, setArticleId] = React.useState(null);
  const openLearn      = () => { setPrevScreen('today'); setScreen('learn'); };
  const openArticle    = (id) => { setArticleId(id); setScreen('article'); };

  // Journal (Самочувствие) navigation + state
  const [journalEntries, setJournalEntries] = React.useState(JOURNAL.ENTRIES);
  const [quickFeelOpen, setQuickFeelOpen] = React.useState(false);
  const openJournal    = () => { setPrevScreen(screen === 'today' ? 'today' : screen); setScreen('journal'); };
  const openQuickFeel  = () => setQuickFeelOpen(true);
  const saveQuickFeel  = (entry) => {
    setJournalEntries(prev => {
      const rest = prev.filter(e => e.day !== JOURNAL.TODAY_DAY);
      return [...rest, { id: `u-${Date.now()}`, day: JOURNAL.TODAY_DAY, ...entry }];
    });
    setQuickFeelOpen(false);
    if (screen !== 'journal') openJournal();
  };

  // Body (Тело) navigation + state
  const [bodyState, setBodyState] = React.useState(() => BODY.seed());
  const openBody       = () => { setPrevScreen(screen === 'today' ? 'today' : screen); setScreen('body'); };
  const addBodyMeasure = (id, val) => {
    setBodyState(prev => ({ ...prev, hist: { ...prev.hist, [id]: [...prev.hist[id], Number(val)] } }));
  };
  const addBodyPhoto   = () => {
    setBodyState(prev => {
      const w = prev.hist.weight[prev.hist.weight.length - 1];
      const week = `Нед ${JOURNAL.weekOf(JOURNAL.TODAY_DAY)}`;
      return { ...prev, photos: [...prev.photos, { id: `u-${Date.now()}`, week, weight: w }] };
    });
  };

  // Recipes (Рецепты) navigation + state
  const [userRecipes, setUserRecipes] = React.useState([]);
  const [recipeId, setRecipeId] = React.useState(null);
  const allRecipes = [...userRecipes, ...RECIPES.STARTERS];
  const openRecipes  = () => { setPrevScreen(screen === 'nutrition' ? 'nutrition' : 'today'); setScreen('recipes'); };
  const openRecipe   = (id) => { setRecipeId(id); setScreen('recipe-detail'); };
  const openBuilder  = () => setScreen('recipe-builder');
  const saveRecipe   = (recipe) => { setUserRecipes(prev => [recipe, ...prev]); setRecipeId(recipe.id); setScreen('recipe-detail'); };
  const addRecipeToDay = (recipe, portions) => {
    const meal = RECIPES.toMeal(recipe, portions, DAY_STATES[dayState].now);
    const newMeal = { id: `u-${Date.now()}`, ...meal };
    const nextMeals = [...meals, newMeal];
    const nextTotals = dayTotals(nextMeals);
    setUserMeals(prev => [...prev, newMeal]);
    setMealCoach(pickMealCoach({ meals: nextMeals, lastMeal: newMeal, totals: nextTotals }));
    setConfirmSheet({ kcal: nextTotals.kcal, mealName: meal.mealName });
    setScreen('nutrition');
    setTimeout(() => setConfirmSheet(null), 1700);
  };

  const goBack = () => {
    if (screen === 'trend-detail') setScreen(prevScreen === 'trends' ? 'trends' : 'today');
    else if (screen === 'trends')  setScreen('today');
    else if (screen === 'nutrition') setScreen('today');
    else if (screen === 'vials') setScreen('today');
    else if (screen === 'add-vial') setScreen('vials');
    else if (screen === 'chat-thread') setScreen(prevScreen === 'chat-list' ? 'chat-list' : 'today');
    else if (screen === 'chat-list')   setScreen('today');
    else if (screen === 'profile')     setScreen('today');
    else if (screen === 'schedule')    setScreen('today');
    else if (screen === 'learn')       setScreen('today');
    else if (screen === 'article')     setScreen('learn');
    else if (screen === 'journal')     setScreen(prevScreen === 'today' ? 'today' : prevScreen);
    else if (screen === 'body')        setScreen(prevScreen === 'today' ? 'today' : prevScreen);
    else if (screen === 'recipes')     setScreen(prevScreen === 'nutrition' ? 'nutrition' : 'today');
    else if (screen === 'recipe-detail') setScreen('recipes');
    else if (screen === 'recipe-builder') setScreen('recipes');
  };

  const handleTabFromOther = (id) => {
    if (id === 'today')     { setScreen('today'); return; }
    if (id === 'log')       { openActionSheet(); return; }
    if (id === 'insights')  { openTrends(); return; }
    if (id === 'inventory') { openVials(); return; }
    if (id === 'nutrition') { openNutrition(); return; }
    setScreen('today');
  };

  // Inventory derived
  const inv = VIAL_INVENTORY;
  const invSummary = inventorySummary(inv);
  const reorderHint = invSummary.reorder[0] || null;
  const detailVial = vialDetailId ? inv.find(v => v.id === vialDetailId) : null;

  // ── Device + palette ────────────────────────────────────────
  const DeviceFrame = platform === 'android' ? AndroidDevice : IOSDevice;
  const FRAME_W = platform === 'android' ? 412 : 390;
  const FRAME_H = platform === 'android' ? 870 : 844;
  const pal = getPalette(false);

  // ── Layers ──────────────────────────────────────────────────
  const today = (
    <V1Refined
      dark={false}
      doseLogged={doseLogged}
      setDoseLogged={setDoseLogged}
      coachIndex={coachIndex}
      platform={platform}
      onLogDose={openLog}
      onPlusTap={openActionSheet}
      onOpenTrends={openTrends}
      onOpenTrend={openTrend}
      onOpenVials={openVials}
      onOpenChat={openChat}
      onOpenProfile={openProfile}
      onOpenSchedule={openSchedule}
      onOpenLearn={openLearn}
      onOpenJournal={openJournal}
      onQuickFeel={openQuickFeel}
      doseJustLogged={doseLogged}
      reorderHint={reorderHint}
      meals={meals}
      mealTotals={mealTotals}
      mealCoach={mealCoach}
      mealHeroSuggestion={mealHeroSuggestion}
      onLogMeal={openMealLog}
      onOpenNutrition={openNutrition}
      onOpenRecipes={openRecipes}
    />
  );

  const trendsOpen = screen === 'trends' || screen === 'trend-detail';
  const trendsLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 40,
      transform: trendsOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: trendsOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <TrendsLanding
        pal={pal}
        platform={platform}
        timeframe={timeframe}
        setTimeframe={(v) => setTweak('timeframe', v)}
        activeBiomarker={biomarker}
        onBack={() => setScreen('today')}
        onOpenDetail={openTrend}
        onChangeTab={handleTabFromOther}
        onOpenJournal={openJournal}
        onOpenBody={openBody}
      />
    </div>
  );

  const detailOpen = screen === 'trend-detail';
  const detailLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 50,
      transform: detailOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: detailOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <TrendDetail
        pal={pal}
        platform={platform}
        biomarkerId={biomarker}
        timeframe={timeframe}
        setTimeframe={(v) => setTweak('timeframe', v)}
        headerVariant={headerVariant}
        showAnnotations={showAnnotations}
        onBack={goBack}
        onSwitchBiomarker={(id) => setTweak('biomarker', id)}
      />
    </div>
  );

  const nutritionOpen = screen === 'nutrition';
  const nutritionLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 42,
      transform: nutritionOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: nutritionOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <NutritionToday
        pal={pal}
        platform={platform}
        meals={meals}
        totals={mealTotals}
        onBack={goBack}
        onLogMeal={openMealLog}
        onChangeTab={handleTabFromOther}
        onOpenRecipes={openRecipes}
      />
    </div>
  );

  // Log-dose modal — slides up
  const logOpen = screen === 'log-dose';
  const logOverlay = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 60,
      transform: logOpen ? 'translateY(0)' : 'translateY(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: logOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <LogDoseV1
        key={logSession}
        dark={false}
        platform={platform}
        onCancel={closeLog}
        onComplete={completeLog}
      />
    </div>
  );

  // Log-meal modal — slides up
  const mealOpen = screen === 'log-meal';
  const mealOverlay = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 60,
      transform: mealOpen ? 'translateY(0)' : 'translateY(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: mealOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <MealLogScreen
        key={mealSession}
        dark={false}
        platform={platform}
        defaultMode={mealMode}
        onCancel={closeMealLog}
        onComplete={completeMealLog}
      />
    </div>
  );

  // Chat layers — slide in from right
  const chatListOpen = screen === 'chat-list' || screen === 'chat-thread';
  const chatListLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 46,
      transform: chatListOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: chatListOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <ChatLanding
        pal={pal}
        platform={platform}
        onBack={() => setScreen('today')}
        onOpenThread={openChatThread}
      />
    </div>
  );

  const chatThreadOpen = screen === 'chat-thread';
  const chatThreadLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 48,
      transform: chatThreadOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: chatThreadOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <ChatConversation
        pal={pal}
        platform={platform}
        threadId={chatThreadId}
        onBack={goBack}
        onBackToList={openChatList}
      />
    </div>
  );

  // Profile — slide in from right (matches Trends / Vials / Chat)
  const profileOpen = screen === 'profile';
  const profileLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 52,
      transform: profileOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: profileOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <ProfileScreen
        pal={pal}
        platform={platform}
        onBack={() => setScreen('today')}
        onOpenChat={openChatFromProfile}
        onOpenTrend={openTrend}
        onOpenSchedule={openSchedule}
        onOpenJournal={openJournal}
        onOpenBody={openBody}
      />
    </div>
  );

  // Schedule — slide in from right
  const scheduleOpen = screen === 'schedule';
  const scheduleLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 54,
      transform: scheduleOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: scheduleOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <ScheduleScreen
        pal={pal}
        platform={platform}
        doseLogged={doseLogged}
        todayMeals={meals ? meals.length : 0}
        todayKcal={mealTotals ? mealTotals.kcal : 0}
        onBack={() => setScreen('today')}
        onLogDose={openLogFromSchedule}
      />
    </div>
  );

  // Learn library + article reader — slide in from right
  const learnOpen = screen === 'learn' || screen === 'article';
  const learnLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 56,
      transform: learnOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: learnOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <LibraryScreen
        pal={pal}
        platform={platform}
        onBack={() => setScreen('today')}
        onOpenArticle={openArticle}
      />
    </div>
  );

  const articleOpen = screen === 'article';
  const articleLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 58,
      transform: articleOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: articleOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <ArticleReader
        pal={pal}
        platform={platform}
        article={articleId ? LEARN.byId(articleId) : null}
        onBack={() => setScreen('learn')}
        onOpenArticle={openArticle}
      />
    </div>
  );

  // Journal (Самочувствие) — slide in from right
  const journalOpen = screen === 'journal';
  const journalLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 57,
      transform: journalOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: journalOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <JournalScreen
        pal={pal}
        platform={platform}
        entries={journalEntries}
        onBack={goBack}
        onQuickAdd={openQuickFeel}
      />
    </div>
  );

  const quickFeelOverlay = quickFeelOpen && (
    <QuickFeelSheet
      pal={pal}
      platform={platform}
      onCancel={() => setQuickFeelOpen(false)}
      onSave={saveQuickFeel}
    />
  );

  // Body (Тело) — slide in from right
  const bodyOpen = screen === 'body';
  const bodyLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 53,
      transform: bodyOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: bodyOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <BodyScreen
        pal={pal}
        platform={platform}
        state={bodyState}
        onBack={goBack}
        onAddMeasure={addBodyMeasure}
        onAddPhoto={addBodyPhoto}
        onOpenTrend={openTrend}
      />
    </div>
  );

  // Recipes — library + detail slide in from right; builder slides up
  const recipesOpen = screen === 'recipes' || screen === 'recipe-detail' || screen === 'recipe-builder';
  const recipesLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 62,
      transform: recipesOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: recipesOpen ? 'auto' : 'none', background: pal.bg,
    }}>
      <RecipeLibrary
        pal={pal}
        platform={platform}
        recipes={allRecipes}
        todayTotals={mealTotals}
        onBack={() => setScreen(prevScreen === 'nutrition' ? 'nutrition' : 'today')}
        onOpen={openRecipe}
        onCreate={openBuilder}
      />
    </div>
  );

  const recipeDetailOpen = screen === 'recipe-detail';
  const recipeDetailLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 64,
      transform: recipeDetailOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: recipeDetailOpen ? 'auto' : 'none', background: pal.bg,
    }}>
      <RecipeDetail
        pal={pal}
        platform={platform}
        recipe={recipeId ? allRecipes.find(r => r.id === recipeId) : null}
        onBack={() => setScreen('recipes')}
        onAddToDay={addRecipeToDay}
      />
    </div>
  );

  const builderOpen = screen === 'recipe-builder';
  const builderLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 66,
      transform: builderOpen ? 'translateY(0)' : 'translateY(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: builderOpen ? 'auto' : 'none', background: pal.bg,
    }}>
      <RecipeBuilder
        pal={pal}
        platform={platform}
        onCancel={() => setScreen('recipes')}
        onSave={saveRecipe}
      />
    </div>
  );

  // Vials landing — slide in from right
  const vialsOpen = screen === 'vials' || screen === 'add-vial';
  const vialsLayer = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 44,
      transform: vialsOpen ? 'translateX(0)' : 'translateX(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: vialsOpen ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <VialsScreen
        pal={pal}
        platform={platform}
        onAddVial={openAddVial}
        onOpenVial={(id) => setVialDetailId(id)}
        onChangeTab={handleTabFromOther}
        sealedOpen={sealedOpen}
        setSealedOpen={setSealedOpen}
      />
    </div>
  );

  // Add-vial modal — slides up
  const addVialOpenNow = screen === 'add-vial';
  const addVialOverlay = (
    <div style={{
      position: 'absolute', inset: 0, zIndex: 60,
      transform: addVialOpenNow ? 'translateY(0)' : 'translateY(100%)',
      transition: 'transform 380ms cubic-bezier(0.22, 1, 0.36, 1)',
      pointerEvents: addVialOpenNow ? 'auto' : 'none',
      background: pal.bg,
    }}>
      <AddVialScreen
        key={addVialSession}
        dark={false}
        platform={platform}
        onCancel={closeAddVial}
        onComplete={completeAddVial}
      />
    </div>
  );

  const confirmOverlay = confirmSheet && (
    <div style={{ position: 'absolute', inset: 0, zIndex: 80 }}>
      <div className="scrim" style={{
        position: 'absolute', inset: 0,
        background: 'rgba(20,44,31,.25)',
        backdropFilter: 'blur(3px)',
      }} />
      <div className="sheet" style={{
        position: 'absolute', left: 16, right: 16, bottom: 24,
        background: pal.bg, borderRadius: 22, padding: 18,
        boxShadow: '0 18px 40px rgba(0,0,0,.22)',
        border: `1px solid ${pal.hairline}`,
        display: 'flex', alignItems: 'center', gap: 14,
      }}>
        <div style={{
          width: 44, height: 44, borderRadius: 999,
          background: C.forest700, color: C.cream,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          flexShrink: 0,
        }}>
          <svg className="tick" width="20" height="20" viewBox="0 0 16 16" fill="none">
            <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{
            fontFamily: F.display, fontSize: 18, color: pal.ink,
            lineHeight: 1.2, letterSpacing: '-0.012em', marginBottom: 2,
          }}>{confirmSheet.mealName} · записано</div>
          <div style={{ fontFamily: F.body, fontSize: 12, color: pal.muted, fontVariantNumeric: 'tabular-nums' }}>
            <span style={{ fontFamily: F.mono, fontWeight: 500, color: pal.ink2 }}>{confirmSheet.kcal.toLocaleString()}</span>
            <span style={{ color: pal.subtle }}> / {MEAL_TARGETS.kcal.toLocaleString()} ккал сегодня</span>
          </div>
        </div>
      </div>
    </div>
  );

  const screenLabel =
    screen === 'log-dose'     ? 'Запись дозы' :
    screen === 'log-meal'     ? 'Запись приёма пищи' :
    screen === 'nutrition'    ? 'Питание' :
    screen === 'trends'       ? 'Тренды' :
    screen === 'trend-detail' ? `Тренды · ${TREND_DATA[biomarker]?.label || biomarker}` :
    screen === 'vials'        ? 'Аптечка' :
    screen === 'add-vial'     ? 'Новый флакон' :
    screen === 'chat-list'    ? 'Команда' :
    screen === 'chat-thread'  ? `Чат · ${(CARE_TEAM.find(t => t.id === chatThreadId) || {}).name || ''}` :
    screen === 'profile'      ? 'Профиль' :
    screen === 'schedule'     ? 'График' :
    screen === 'learn'        ? 'Знания' :
    screen === 'journal'      ? 'Самочувствие' :
    screen === 'body'         ? 'Тело' :
    screen === 'recipes'      ? 'Рецепты' :
    screen === 'recipe-detail' ? `Рецепт · ${(allRecipes.find(r => r.id === recipeId) || {}).name || ''}` :
    screen === 'recipe-builder' ? 'Новый рецепт' :
    screen === 'article'      ? `Знания · ${(LEARN.byId(articleId) || {}).eyebrow || ''}` :
                                'Сегодня';

  return (
    <>
      <div style={{
        minHeight: '100vh', background: '#2a2a2a', color: '#f6f1ea',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: '32px 20px', boxSizing: 'border-box',
      }}>
        <div style={{ position: 'relative' }}>
          <DeviceFrame width={FRAME_W} height={FRAME_H}>
            <div data-screen-label={screenLabel} style={{ position: 'relative', height: '100%', background: pal.bg }}>
              {today}
              {trendsLayer}
              {nutritionLayer}
              {vialsLayer}
              {chatListLayer}
              {chatThreadLayer}
              {profileLayer}
              {scheduleLayer}
              {learnLayer}
              {articleLayer}
              {journalLayer}
              {bodyLayer}
              {recipesLayer}
              {recipeDetailLayer}
              {builderLayer}
              {detailLayer}
              {logOverlay}
              {mealOverlay}
              {addVialOverlay}
              {quickFeelOverlay}
              {confirmOverlay}
              {actionSheetOpen && (
                <ActionChooserSheet
                  pal={pal}
                  doseLogged={doseLogged}
                  meals={meals}
                  mealTotals={mealTotals}
                  onClose={closeActionSheet}
                  onPickDose={() => { closeActionSheet(); openLog(); }}
                  onPickMeal={() => { closeActionSheet(); openMealLog(); }}
                />
              )}
              <VialDetailSheet
                open={!!vialDetailId}
                vial={detailVial}
                pal={pal}
                onClose={() => setVialDetailId(null)}
                onMarkOpened={(id) => { setVialDetailId(null); }}
                onLogDoseFromVial={(id) => { setVialDetailId(null); openLog(); }}
                onEdit={(id) => setVialDetailId(null)}
                onAddPhoto={(id) => setVialDetailId(null)}
                onDispose={(id) => setVialDetailId(null)}
                onMoveToSpare={(id) => setVialDetailId(null)}
                onActivate={(id) => setVialDetailId(null)}
              />
            </div>
          </DeviceFrame>

          <div style={{
            position: 'absolute', left: '50%', transform: 'translateX(-50%)',
            bottom: -28, whiteSpace: 'nowrap',
            fontFamily: 'var(--font-mono)', fontSize: 11, color: 'rgba(246,241,234,.6)',
            letterSpacing: '.04em',
          }}>
            Cadence · {platform === 'android' ? 'Pixel 8' : 'iPhone 15'} · {screenLabel}
          </div>
        </div>
      </div>

      {/* Tweaks panel intentionally empty — keeps the host protocol wired
          so toggling Tweaks mode still works, even though nothing is exposed. */}
      <TweaksPanel />
    </>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<PrototypeApp />);

// ─────────────────────────────────────────────────────────────────
// Action chooser sheet — opens when the user taps the '+' tab.
// Two tappable options: log a dose, log a meal.
// ─────────────────────────────────────────────────────────────────

function ActionChooserSheet({ pal, doseLogged, meals, mealTotals, onClose, onPickDose, onPickMeal }) {
  const mealCount = meals ? meals.length : 0;
  const mealKcal = mealTotals ? mealTotals.kcal : 0;

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 80 }}>
      <div className="scrim" onClick={onClose} style={{
        position: 'absolute', inset: 0,
        background: 'rgba(20,44,31,.35)',
        backdropFilter: 'blur(4px)',
      }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        padding: '12px 20px 32px',
        boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{
          width: 38, height: 4, borderRadius: 999, background: pal.border,
          margin: '0 auto 16px',
        }} />

        <div style={{ marginBottom: 18 }}>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>Что записываем?</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 28, color: pal.ink,
            lineHeight: 1.04, letterSpacing: '-0.018em',
          }}>
            Выберите <span style={{ fontStyle: 'italic', color: C.forest700 }}>ритм</span>.
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 14 }}>
          <ActionOption
            iconBg={C.forest700}
            iconFg={C.cream}
            iconName="beaker"
            title="Записать дозу"
            sub={doseLogged ? 'Уже записано сегодня · открыть или поправить' : 'Семаглутид · 0,25 мг ждёт'}
            pal={pal}
            onClick={onPickDose}
          />
          <ActionOption
            iconBg={C.sand500}
            iconFg={C.ink900}
            iconName="cake"
            title="Записать приём пищи"
            sub={mealCount === 0
              ? 'Пока ничего сегодня · начнём ритм'
              : `${mealCount} ${mealCount === 1 ? 'приём' : mealCount < 5 ? 'приёма' : 'приёмов'} сегодня · ${mealKcal.toLocaleString()} ккал`}
            pal={pal}
            onClick={onPickMeal}
          />
        </div>

        <button
          onClick={onClose}
          className="press"
          style={{
            width: '100%', padding: '13px 16px', borderRadius: 999,
            background: 'transparent', border: `1px solid ${pal.border}`,
            color: pal.muted, fontFamily: F.body, fontSize: 13, fontWeight: 500,
            cursor: 'pointer',
          }}
        >
          Отмена
        </button>
      </div>
    </div>
  );
}

function ActionOption({ iconBg, iconFg, iconName, title, sub, pal, onClick }) {
  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        display: 'grid', gridTemplateColumns: '52px 1fr auto',
        gap: 14, alignItems: 'center',
        width: '100%', padding: 14,
        background: pal.paper, border: `1px solid ${pal.hairline}`,
        borderRadius: 18, cursor: 'pointer',
        boxShadow: '0 2px 6px rgba(46,38,24,.05)',
        textAlign: 'left',
      }}
    >
      <div style={{
        width: 52, height: 52, borderRadius: 14,
        background: iconBg, color: iconFg,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name={iconName} size={24} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{
          fontFamily: F.display, fontSize: 22, color: pal.ink,
          lineHeight: 1.1, letterSpacing: '-0.012em',
        }}>{title}</div>
        <div style={{
          fontFamily: F.body, fontSize: 12, color: pal.muted, marginTop: 3,
        }}>{sub}</div>
      </div>
      <Icon name="chevron-right" size={18} color={pal.subtle} />
    </button>
  );
}
