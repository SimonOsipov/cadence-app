// Cadence · Рецепты (Recipe creator) — data model
// A small ingredient database + GLP-1-friendly, protein-forward starter recipes,
// plus pure helpers for macro math, personalization, and logging to the day.
// Exposes: RECIPES on window. Reuses MEAL_TARGETS from meal-data.

(function () {
  // ── Ingredient DB (per 100 g) ────────────────────────────
  const ING = [
    { id: 'chicken',  name: 'Куриная грудка',     kcal: 165, p: 31,  c: 0,   f: 3.6 },
    { id: 'turkey',   name: 'Индейка, филе',      kcal: 135, p: 29,  c: 0,   f: 1.5 },
    { id: 'salmon',   name: 'Лосось',             kcal: 208, p: 20,  c: 0,   f: 13  },
    { id: 'tuna',     name: 'Тунец',              kcal: 116, p: 26,  c: 0,   f: 1   },
    { id: 'shrimp',   name: 'Креветки',           kcal: 99,  p: 24,  c: 0.2, f: 0.3 },
    { id: 'egg',      name: 'Яйцо',               kcal: 155, p: 13,  c: 1.1, f: 11  },
    { id: 'cottage',  name: 'Творог 5%',          kcal: 121, p: 17,  c: 3,   f: 5   },
    { id: 'greekyog', name: 'Греческий йогурт',   kcal: 59,  p: 10,  c: 3.6, f: 0.4 },
    { id: 'kefir',    name: 'Кефир 1%',           kcal: 40,  p: 3.4, c: 4,   f: 1   },
    { id: 'tofu',     name: 'Тофу',               kcal: 144, p: 17,  c: 2.8, f: 9   },
    { id: 'whey',     name: 'Протеин, порошок',   kcal: 380, p: 75,  c: 8,   f: 6   },
    { id: 'rice',     name: 'Бурый рис, варёный', kcal: 123, p: 2.7, c: 25,  f: 1   },
    { id: 'oats',     name: 'Овсянка, сухая',     kcal: 379, p: 13,  c: 67,  f: 7   },
    { id: 'quinoa',   name: 'Киноа, варёная',     kcal: 120, p: 4.4, c: 21,  f: 1.9 },
    { id: 'lentils',  name: 'Чечевица, варёная',  kcal: 116, p: 9,   c: 20,  f: 0.4 },
    { id: 'sweetpot', name: 'Батат',              kcal: 86,  p: 1.6, c: 20,  f: 0.1 },
    { id: 'broccoli', name: 'Брокколи',           kcal: 34,  p: 2.8, c: 7,   f: 0.4 },
    { id: 'spinach',  name: 'Шпинат',             kcal: 23,  p: 2.9, c: 3.6, f: 0.4 },
    { id: 'avocado',  name: 'Авокадо',            kcal: 160, p: 2,   c: 9,   f: 15  },
    { id: 'banana',   name: 'Банан',              kcal: 89,  p: 1.1, c: 23,  f: 0.3 },
    { id: 'berries',  name: 'Ягоды',              kcal: 57,  p: 0.7, c: 14,  f: 0.3 },
    { id: 'almond',   name: 'Миндаль',            kcal: 579, p: 21,  c: 22,  f: 50  },
    { id: 'tahini',   name: 'Тахини',             kcal: 595, p: 17,  c: 21,  f: 54  },
    { id: 'oliveoil', name: 'Оливковое масло',    kcal: 884, p: 0,   c: 0,   f: 100 },
    { id: 'peanut',   name: 'Арахисовая паста',   kcal: 588, p: 25,  c: 20,  f: 50  },
  ];
  const ingMeta = (id) => ING.find(i => i.id === id);

  // ── Taxonomy ─────────────────────────────────────────────
  const TAGS = [
    { id: 'protein', label: 'Белковые' },
    { id: 'gentle',  label: 'Мягкие для желудка' },
    { id: 'quick',   label: 'Быстрые' },
  ];
  const MEAL_TYPES = [
    { id: 'breakfast', label: 'Завтрак' },
    { id: 'lunch',     label: 'Обед' },
    { id: 'dinner',    label: 'Ужин' },
    { id: 'snack',     label: 'Перекус' },
  ];
  const tagLabel = (id) => (TAGS.find(t => t.id === id) || {}).label || id;
  const typeLabel = (id) => (MEAL_TYPES.find(t => t.id === id) || {}).label || id;

  // ── Starter recipes (fully written) ──────────────────────
  const STARTERS = [
    {
      id: 'chicken-bowl', name: 'Боул с курицей и тахини', mealType: 'lunch',
      tags: ['protein'], servings: 2, prepMin: 15, cookMin: 20, photo: true, photoSlot: 'recipe-feat',
      dek: 'Опорный обед курса: много белка, тёплые овощи и капля тахини для вкуса.',
      ingredients: [
        { id: 'chicken', g: 300 }, { id: 'rice', g: 240 }, { id: 'broccoli', g: 160 },
        { id: 'sweetpot', g: 200 }, { id: 'tahini', g: 30 }, { id: 'oliveoil', g: 10 },
      ],
      steps: [
        'Нарежьте куриную грудку полосками, посолите и оставьте на 10 минут.',
        'Батат кубиками, брокколи соцветиями — запеките при 200 °C около 20 минут с ложкой масла.',
        'Обжарьте курицу на сухой сковороде до золотистого, 6–8 минут.',
        'Соберите боул: рис, овощи, курица. Полейте тахини, разведённой ложкой воды.',
      ],
    },
    {
      id: 'cottage-bowl', name: 'Творожная чаша с ягодами', mealType: 'breakfast',
      tags: ['protein', 'gentle', 'quick'], servings: 1, prepMin: 5, cookMin: 0, photo: false,
      dek: 'Пять минут, 30+ грамм белка и ничего тяжёлого для утра.',
      ingredients: [
        { id: 'cottage', g: 200 }, { id: 'berries', g: 80 }, { id: 'banana', g: 60 }, { id: 'almond', g: 15 },
      ],
      steps: [
        'Выложите творог в миску, разровняйте.',
        'Сверху — ягоды и нарезанный банан.',
        'Присыпьте рубленым миндалём. По желанию — ложка кефира для нежности.',
      ],
    },
    {
      id: 'spinach-omelette', name: 'Омлет со шпинатом и творогом', mealType: 'breakfast',
      tags: ['protein', 'gentle', 'quick'], servings: 1, prepMin: 5, cookMin: 8, photo: false,
      dek: 'Мягкий, воздушный, садится легко даже в тошнотные дни.',
      ingredients: [
        { id: 'egg', g: 150 }, { id: 'spinach', g: 50 }, { id: 'cottage', g: 50 }, { id: 'oliveoil', g: 5 },
      ],
      steps: [
        'Взбейте яйца с щепоткой соли.',
        'Припустите шпинат на капле масла, 1 минуту.',
        'Влейте яйца, на среднем огне доведите до мягкости.',
        'В центр — ложку творога, сложите омлет пополам.',
      ],
    },
    {
      id: 'salmon-quinoa', name: 'Лосось с киноа и брокколи', mealType: 'dinner',
      tags: ['protein'], servings: 2, prepMin: 10, cookMin: 18, photo: false,
      dek: 'Спокойный ужин: жиры из лосося и ровный белок без тяжести.',
      ingredients: [
        { id: 'salmon', g: 300 }, { id: 'quinoa', g: 300 }, { id: 'broccoli', g: 200 }, { id: 'oliveoil', g: 16 },
      ],
      steps: [
        'Лосось посолите, сбрызните маслом и запеките при 190 °C, 14–16 минут.',
        'Брокколи отварите на пару 5 минут — до яркого зелёного.',
        'Подавайте на тёплой киноа, рыбу сверху.',
      ],
    },
    {
      id: 'lentil-turkey', name: 'Тёплая чечевица с индейкой', mealType: 'dinner',
      tags: ['protein', 'gentle'], servings: 2, prepMin: 10, cookMin: 20, photo: false,
      dek: 'Сытно и мягко: клетчатка чечевицы и постная индейка.',
      ingredients: [
        { id: 'turkey', g: 240 }, { id: 'lentils', g: 300 }, { id: 'spinach', g: 60 }, { id: 'oliveoil', g: 16 },
      ],
      steps: [
        'Индейку нарежьте кубиками, обжарьте на ложке масла до готовности.',
        'Добавьте варёную чечевицу, прогрейте вместе 3–4 минуты.',
        'В конце вмешайте шпинат, дайте ему опасть. Посолите.',
      ],
    },
    {
      id: 'protein-smoothie', name: 'Протеиновый смузи', mealType: 'snack',
      tags: ['protein', 'gentle', 'quick'], servings: 1, prepMin: 3, cookMin: 0, photo: false,
      dek: 'Когда жевать не хочется: белок в стакане, мягко и быстро.',
      ingredients: [
        { id: 'whey', g: 30 }, { id: 'kefir', g: 200 }, { id: 'banana', g: 80 }, { id: 'peanut', g: 15 },
      ],
      steps: [
        'Сложите всё в блендер.',
        'Взбейте до гладкости, 30 секунд.',
        'Густо — добавьте воды или льда по вкусу.',
      ],
    },
  ];

  // ── Macro math ───────────────────────────────────────────
  function ingMacros(id, grams) {
    const m = ingMeta(id); if (!m) return { kcal: 0, p: 0, c: 0, f: 0 };
    const r = grams / 100;
    return { kcal: m.kcal * r, p: m.p * r, c: m.c * r, f: m.f * r };
  }
  // whole-recipe totals
  function recipeTotals(recipe) {
    return recipe.ingredients.reduce((a, it) => {
      const m = ingMacros(it.id, it.g);
      return { kcal: a.kcal + m.kcal, p: a.p + m.p, c: a.c + m.c, f: a.f + m.f };
    }, { kcal: 0, p: 0, c: 0, f: 0 });
  }
  function perServing(recipe) {
    const t = recipeTotals(recipe), s = recipe.servings || 1;
    return { kcal: t.kcal / s, p: t.p / s, c: t.c / s, f: t.f / s };
  }
  const round = (n) => Math.round(n);
  const round1 = (n) => Math.round(n * 10) / 10;
  function totalTime(r) { return (r.prepMin || 0) + (r.cookMin || 0); }

  // ── Personalization ──────────────────────────────────────
  function remaining(todayTotals) {
    const t = window.MEAL_TARGETS;
    const tt = todayTotals || { kcal: 0, p: 0, c: 0, f: 0 };
    return {
      kcal: Math.max(0, t.kcal - tt.kcal),
      p:    Math.max(0, t.protein - tt.p),
    };
  }
  // pick the recipe that best fills the remaining protein without busting kcal
  function suggest(recipes, todayTotals) {
    const rem = remaining(todayTotals);
    const scored = recipes.map(r => {
      const ps = perServing(r);
      const fits = ps.kcal <= rem.kcal + 250;          // soft kcal ceiling
      const proteinScore = ps.p;                        // protein-forward
      return { r, ps, score: proteinScore - (fits ? 0 : 40) };
    }).sort((a, b) => b.score - a.score);
    return { pick: scored[0] ? scored[0].r : recipes[0], rem };
  }
  function filter(recipes, { tag, mealType }) {
    return recipes.filter(r =>
      (!tag || (r.tags || []).includes(tag)) &&
      (!mealType || r.mealType === mealType));
  }

  // ── Build a day-meal from a recipe (×portions) ───────────
  function toMeal(recipe, portions, time) {
    const factor = portions / (recipe.servings || 1);
    const items = recipe.ingredients.map((it, i) => {
      const m = ingMacros(it.id, it.g * factor);
      return {
        id: `${recipe.id}-${i}`, name: ingMeta(it.id).name,
        grams: round(it.g * factor),
        kcal: round(m.kcal), p: round1(m.p), c: round1(m.c), f: round1(m.f),
      };
    });
    const totals = items.reduce((a, it) => ({ kcal: a.kcal + it.kcal, p: a.p + it.p, c: a.c + it.c, f: a.f + it.f }), { kcal: 0, p: 0, c: 0, f: 0 });
    return { time: time || '', mealName: recipe.name, items, totals };
  }

  window.RECIPES = {
    ING, ingMeta, TAGS, MEAL_TYPES, tagLabel, typeLabel, STARTERS,
    ingMacros, recipeTotals, perServing, totalTime, round, round1,
    remaining, suggest, filter, toMeal,
  };
})();
