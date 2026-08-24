package main

// The people the screens were designed around, and the reason they are written
// out rather than generated: the dashboard's roster fixture
// (web/src/data/fixtures/overview.ts) and the mobile persona are what the design
// was drawn against, and a seeded clinic showing other names is one nobody can
// hold up against it.

// staffMember is one member of the care team. Every one of them is created as a
// doctor — migration 000006 lets the service path write no other staff role —
// and careRole is what they do for a patient they are put on.
type staffMember struct {
	slug     string
	fullName string
	title    string
	careRole string
}

// body is what the clinic measured about a patient. Present for the persona the
// mobile screens read; absent for the roster, where the design states nothing
// but a name and an age, and a guessed height is a measurement nobody took.
type body struct {
	sex            string
	heightCM       float64
	targetWeightKG float64
}

// seededPatient is one row of the roster. careTeam names members of staff by
// slug, and the first of them is the primary specialist.
type seededPatient struct {
	slug     string
	fullName string
	age      int
	careTeam []string
	body     *body

	// signsIn: they are given a password, which confirms their address and makes
	// their invitation read as accepted. The rest are invited and nothing more —
	// which is both what pending means and what most of a roster looks like on
	// any given day.
	signsIn bool

	// prescribed: the roster is a fixture of names and ages, and a course invented
	// for each of them is treatment nobody prescribed.
	prescribed bool
}

type clinic struct {
	staff    []staffMember
	patients []seededPatient
}

// theClinic is the whole seed as data: nothing below decides who exists.
func theClinic() clinic {
	return clinic{staff: careTeam, patients: append([]seededPatient{marina}, roster...)}
}

// The three the mobile chat screen draws, with the specialities it names them by.
var careTeam = []staffMember{
	{slug: "ksenia", fullName: "Ксения Первеева", title: "Эндокринолог", careRole: "endo"},
	{slug: "maria", fullName: "Мария Светова", title: "Диетолог", careRole: "dietitian"},
	{slug: "tatiana", fullName: "Татьяна Лесова", title: "Медсестра", careRole: "nurse"},
}

// marina is the persona every mobile screen was drawn around. She gets a
// password because the app has to open as somebody.
//
// The prototype states no date of birth, so the age here is the seed's own. The
// height is not: 110 кг at the ИМТ 31,2 her profile screen shows is 188 см, and
// the ↓2,1 beside it against her 118 кг start agrees.
var marina = seededPatient{
	slug:       "marina-volkova",
	fullName:   "Марина Волкова",
	age:        38,
	careTeam:   []string{"ksenia", "maria", "tatiana"},
	body:       &body{sex: "female", heightCM: 188, targetWeightKG: 102},
	signsIn:    true,
	prescribed: true,
}

// roster is the dashboard's own fixture, name and age, every one of them the
// endocrinologist's patient — the doctor that fixture signs in as.
//
// Three of them are given passwords, so that the registry shows both states this
// command can produce. The third state, expired, cannot be: it is measured from
// the moment the provider recorded the invitation, and moving that is reaching
// into the provider's own table.
var roster = []seededPatient{
	{slug: "marina", fullName: "Марина Левченко", age: 41, careTeam: []string{"ksenia"}, signsIn: true},
	{slug: "oleg", fullName: "Олег Самойлов", age: 47, careTeam: []string{"ksenia"}, signsIn: true},
	{slug: "sofia", fullName: "София Ермакова", age: 35, careTeam: []string{"ksenia"}, signsIn: true},
	{slug: "dmitri", fullName: "Дмитрий Орлов", age: 52, careTeam: []string{"ksenia"}},
	{slug: "anna", fullName: "Анна Кравцова", age: 38, careTeam: []string{"ksenia"}},
	{slug: "pavel", fullName: "Павел Гордеев", age: 44, careTeam: []string{"ksenia"}},
	{slug: "irina", fullName: "Ирина Соколова", age: 49, careTeam: []string{"ksenia"}},
	{slug: "viktor", fullName: "Виктор Зайцев", age: 56, careTeam: []string{"ksenia"}},
	{slug: "elena", fullName: "Елена Власова", age: 36, careTeam: []string{"ksenia"}},
	{slug: "roman", fullName: "Роман Беляев", age: 43, careTeam: []string{"ksenia"}},
	{slug: "natalia", fullName: "Наталья Демина", age: 39, careTeam: []string{"ksenia"}},
	{slug: "andrei", fullName: "Андрей Тихонов", age: 50, careTeam: []string{"ksenia"}},
	{slug: "yulia", fullName: "Юлия Фомина", age: 33, careTeam: []string{"ksenia"}},
	{slug: "maxim", fullName: "Максим Корнев", age: 46, careTeam: []string{"ksenia"}},
	{slug: "galina", fullName: "Галина Орехова", age: 54, careTeam: []string{"ksenia"}},
	{slug: "sergei", fullName: "Сергей Лапин", age: 41, careTeam: []string{"ksenia"}},
	{slug: "kira", fullName: "Кира Жукова", age: 37, careTeam: []string{"ksenia"}},
	{slug: "boris", fullName: "Борис Шевцов", age: 58, careTeam: []string{"ksenia"}},
	{slug: "vera", fullName: "Вера Зорина", age: 45, careTeam: []string{"ksenia"}},
	{slug: "timur", fullName: "Тимур Аскеров", age: 42, careTeam: []string{"ksenia"}},
	{slug: "lidia", fullName: "Лидия Панова", age: 51, careTeam: []string{"ksenia"}},
	{slug: "egor", fullName: "Егор Власенко", age: 39, careTeam: []string{"ksenia"}},
	{slug: "alina", fullName: "Алина Серова", age: 34, careTeam: []string{"ksenia"}},
	{slug: "nikita", fullName: "Никита Громов", age: 48, careTeam: []string{"ksenia"}},
	{slug: "darya", fullName: "Дарья Котова", age: 40, careTeam: []string{"ksenia"}},
}
