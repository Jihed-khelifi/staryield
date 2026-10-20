package astro

// Sentence banks for Daily.
//
// Voice rules (see the Staryield brand guide): compassionate, clear and
// non-deterministic. Describe perspective, not destiny; preserve the reader's
// agency; no fear, urgency, judgment or claims of certainty; no medical,
// legal or financial advice. Every sentence must end with a single ".", "?"
// or "!" and contain no other sentence-ending punctuation, so that readings
// stay within their intended length.
//
// Placeholders: {sign}, {moon_sign}, {phase}, {phase_noun}, {moon_theme},
// {phase_meaning}, {element}, {a_element}, {moon_element}, {a_moon_element}.

var openers = []string{
	"Today offers you, {sign}, a gentle chance to notice what feels most alive in you.",
	"There is room today, {sign}, to move at a pace that feels kind to you.",
	"This day may carry a quiet invitation, {sign}, to listen to what you already know.",
	"Something in the air today could encourage you to look at familiar situations with fresh eyes.",
	"You may find, {sign}, that small moments hold more meaning than usual today.",
	"Today can be a soft reminder that you are allowed to take things one step at a time.",
	"The sky today paints a reflective picture, {sign}, one you are free to interpret in your own way.",
	"If you pause for a moment today, you might notice a subtle shift in how you see things.",
	"Today may feel like an open page, {sign}, ready for whatever you choose to bring to it.",
	"A curious, open energy surrounds the day, inviting you to explore rather than rush.",
	"Today could be a lovely moment, {sign}, to check in with yourself before checking in with the world.",
	"There may be a calm thread running through today, {sign}, waiting for you to notice it.",
}

var moonLines = []string{
	"Today's {phase_noun} travels through {moon_sign}, gently highlighting {moon_theme}.",
	"With a {phase_noun} in {moon_sign}, themes of {moon_theme} may feel closer to the surface.",
	"The Moon rests in {moon_sign} today, and its {phase} phase is often seen as {phase_meaning}.",
	"As the {phase_noun} moves through {moon_sign}, you might feel drawn toward {moon_theme}.",
	"A {phase_noun} in {moon_sign} colors the day, a pairing many read as {phase_meaning}.",
	"The Moon's journey through {moon_sign} meets its {phase} phase today, softly inviting {moon_theme}.",
	"Under a {phase_noun} in {moon_sign}, the emotional weather may lean toward {moon_theme}.",
	"The {phase_noun} settles into {moon_sign} today, offering {phase_meaning}.",
	"Today the Moon is in {moon_sign} and in its {phase} phase, which can be {phase_meaning}.",
}

// moonThemes are noun phrases describing what the Moon's sign may color.
var moonThemes = map[string][]string{
	"Aries":       {"courage and honest first steps", "fresh energy and direct expression"},
	"Taurus":      {"comfort, patience and simple pleasures", "steadiness and sensory comfort"},
	"Gemini":      {"curiosity and lively conversation", "learning, ideas and playful connection"},
	"Cancer":      {"home, belonging and emotional care", "tenderness and nurturing connections"},
	"Leo":         {"creativity, warmth and self-expression", "generosity and heartfelt play"},
	"Virgo":       {"thoughtful routines and practical care", "clarity, order and small improvements"},
	"Libra":       {"balance, harmony and partnership", "fairness, beauty and gentle diplomacy"},
	"Scorpio":     {"depth, honesty and emotional truth", "intimacy and quiet transformation"},
	"Sagittarius": {"exploration, optimism and meaning", "big-picture thinking and a spirit of adventure"},
	"Capricorn":   {"structure, patience and long-term hopes", "responsibility and steady progress"},
	"Aquarius":    {"originality, friendship and fresh perspectives", "community and independent thinking"},
	"Pisces":      {"intuition, compassion and dreaming", "imagination and soft reflection"},
}

// phaseMeanings are noun phrases describing how each phase is often read.
var phaseMeanings = map[string][]string{
	PhaseNewMoon: {
		"a quiet moment for setting intentions",
		"an invitation to begin again with a clean page",
		"a time for listening inward before acting",
	},
	PhaseWaxingCrescent: {
		"a time for nurturing small beginnings",
		"an invitation to take one modest step forward",
		"a phase for building gentle momentum",
	},
	PhaseFirstQuarter: {
		"a moment for meeting challenges with flexibility",
		"an invitation to make a thoughtful choice",
		"a time for adjusting plans with an open mind",
	},
	PhaseWaxingGibbous: {
		"a time for refining and fine-tuning",
		"an invitation to stay patient with what is growing",
		"a phase for polishing rather than starting over",
	},
	PhaseFullMoon: {
		"a time when feelings and insights can become clearer",
		"an invitation to celebrate how far you have come",
		"a moment for noticing what has come to light",
	},
	PhaseWaningGibbous: {
		"a time for sharing what you have learned",
		"an invitation to express gratitude",
		"a phase for integrating recent insights",
	},
	PhaseLastQuarter: {
		"a moment for releasing what no longer fits",
		"an invitation to clear a little space",
		"a time for honest review and letting go",
	},
	PhaseWaningCrescent: {
		"a time for rest and quiet reflection",
		"an invitation to slow down and restore",
		"a phase for closing chapters gently",
	},
}

// elementLines speak to the reader's own element.
var elementLines = map[string][]string{
	Fire: {
		"Your natural spark may want to leap ahead, and it could help to channel that warmth into one meaningful project.",
		"As a fire sign, you might feel a pull toward action, and noticing what truly excites you can guide where that energy goes.",
		"Your enthusiasm can be a gift to the people around you, especially when it is shared with patience.",
		"There may be a chance to lead by example today, simply by being honest about what inspires you.",
		"Your courage tends to shine brightest when you follow curiosity rather than pressure.",
		"A playful approach could open doors that effort alone might not.",
		"You might enjoy turning a routine task into a small creative adventure.",
		"Your warmth can light up a conversation, and listening closely may make that glow even brighter.",
		"If restlessness appears, a walk, a favorite song or a bit of movement could help it settle into focus.",
	},
	Earth: {
		"Your grounded nature may appreciate simple, tangible progress today, however small it looks.",
		"As an earth sign, you might find comfort in routines that steady you and give the day a gentle shape.",
		"There may be quiet satisfaction in tending to something practical, like tidying a space or finishing a lingering task.",
		"Your patience is a real strength, and it may help you see value where others might hurry past.",
		"You could explore what it would feel like to rest without first earning it.",
		"Small, steady choices may feel more nourishing today than dramatic changes.",
		"Your reliability is often a comfort to others, and it is equally fine to let someone support you.",
		"Spending a few moments with nature, texture or a favorite meal could help you feel more present.",
		"Your practical wisdom might help you sort what matters now from what can wait.",
	},
	Air: {
		"Your curious mind may light up around new ideas, and a meaningful conversation could spark fresh insight.",
		"As an air sign, you might enjoy connecting dots that others overlook.",
		"Writing down a passing thought could help you see a situation more clearly.",
		"Your gift for perspective may help someone feel understood today.",
		"You might find that asking an open question leads somewhere more interesting than having the answer.",
		"A change of scenery, even a small one, could refresh your thinking.",
		"Your sociable side may enjoy reaching out to someone you have been meaning to contact.",
		"Balancing time with others and time alone could help your ideas settle into something clear.",
		"Your lightness of touch can turn a tense moment into an easier one.",
	},
	Water: {
		"Your intuition may feel especially attuned today, offering quiet hints worth noticing.",
		"As a water sign, you might sense the moods around you, so it can help to check in with your own feelings too.",
		"Your capacity for empathy is a gift, and gentle boundaries can help it stay sustainable.",
		"A moment of stillness, perhaps near water or with soft music, could help you reconnect with yourself.",
		"Your emotional depth may help you understand a situation in a way words cannot quite capture.",
		"Allowing feelings to come and go, without judging them, might bring a sense of ease.",
		"You may feel drawn to the comfort of familiar people and places, which can be deeply restoring.",
		"Your imagination could offer creative answers to a question you have been carrying.",
		"Being as tender with yourself as you are with others could be a meaningful practice today.",
	},
}

// relationLines describe how the Moon's element sits with the reader's.
var relationLines = map[string][]string{
	relationSame: {
		"Because the Moon shares your {element} element today, your instincts and the mood of the day may feel unusually in tune.",
		"With the Moon in a fellow {element} sign, your natural rhythm may feel a little easier to follow.",
		"The Moon sharing your element can make it easier than usual to trust your own pace.",
		"When the Moon moves through {a_element} sign like your own, feelings and actions may flow in the same direction.",
	},
	relationSupportive: {
		"The Moon's {moon_element} energy tends to support your {element} nature, which could make collaboration feel lighter.",
		"There is a supportive blend between the {moon_element} Moon and your {element} nature, a pleasant backdrop for connection.",
		"The {moon_element} tone of today's Moon can complement your {element} temperament, like two instruments finding the same key.",
		"Your {element} nature and the Moon in {a_moon_element} sign may get along well, which can make cooperation feel natural.",
	},
	relationContrast: {
		"The Moon's {moon_element} mood differs from your {element} nature, which can be an interesting chance to see things from another angle.",
		"Today's {moon_element} Moon offers a contrast to your {element} temperament, and that contrast might show you something new about yourself.",
		"With the Moon in {a_moon_element} sign, your {element} nature may be invited to stretch a little, and you get to decide how far feels comfortable.",
		"Different elemental tones between the Moon and your sign could bring a little friction, and with it, room to grow.",
	},
}

var genericLines = []string{
	"In your relationships, a little extra patience could turn an ordinary exchange into a meaningful one.",
	"In work or daily tasks, focusing on one thing at a time may feel more satisfying than juggling many.",
	"It may be a helpful day to notice what drains you and what restores you, without needing to change everything at once.",
	"A kind word, offered or received, could carry more weight than it first seems.",
	"If a decision is on your mind, you can give yourself permission to gather more information before choosing.",
	"Creative pursuits, even simple ones like doodling or cooking, might feel especially rewarding.",
	"You might enjoy reconnecting with a hobby or interest that once brought you joy.",
	"Honest, gentle communication could help clear up a small misunderstanding.",
	"Taking a few slow breaths before responding to something could give you space to answer from a calm place.",
	"Celebrating a small win, even privately, may lift your spirits more than you would think.",
	"Someone close to you may appreciate hearing how much they mean to you.",
	"Leaving a little unstructured time in your day could make room for unexpected inspiration.",
}

var closers = []string{
	"Explore what this pattern might mean for you.",
	"Take what resonates, and leave the rest.",
	"You remain the author of your own story, and this is simply one perspective to reflect on.",
	"Let this be a gentle prompt for reflection rather than a prediction.",
	"Notice what feels true for you, and trust your own sense of timing.",
	"Consider this an invitation to reflect, not a map you need to follow.",
	"Whatever you choose, kindness toward yourself is a lovely place to start.",
	"Reflect on how this energy shows up in your own life, in your own way.",
	"There is no right or wrong way to meet the day, and your choices remain yours.",
	"Hold these ideas lightly and see which ones feel meaningful to you.",
}

var genericFocus = []string{
	"What would it look like to give yourself the same patience you offer others today?",
	"Which small moment today could you savor a little more slowly?",
	"Where might a gentle pause help you choose with more clarity?",
	"What is one thing you could set down today to make room for ease?",
	"Who in your life might enjoy hearing from you, and what would you like to say?",
	"What does feeling supported look like for you right now?",
	"Which part of your day feels most like you, and how might you invite more of it?",
	"What is quietly asking for your attention beneath the busyness?",
}

var elementFocus = map[string][]string{
	Fire: {
		"Where could your enthusiasm be most welcome today?",
		"What would you try if you approached the day as play?",
		"Which spark of inspiration would you like to follow a little further?",
		"How might you share your warmth while keeping some for yourself?",
	},
	Earth: {
		"What simple ritual helps you feel grounded, and could you make room for it today?",
		"Which small step would feel steady and satisfying right now?",
		"What would rest look like if you did not need to earn it?",
		"Where could you trust the slow pace of growth?",
	},
	Air: {
		"Which idea keeps returning to you, and what might it be pointing toward?",
		"What question would you like to explore with genuine curiosity?",
		"Who could you learn something new from today?",
		"How might you balance connection with a little quiet time?",
	},
	Water: {
		"What is your intuition gently telling you today?",
		"How can you honor your feelings without needing to fix them?",
		"Where might a softer boundary, or a firmer one, help you feel at ease?",
		"What brings you comfort, and how might you offer some of it to yourself?",
	},
}

var affirmations = []string{
	"I move through the day at a pace that honors my energy.",
	"I trust myself to choose what feels right for me.",
	"I am allowed to grow in my own time.",
	"My feelings are valid, and I can meet them with kindness.",
	"I make room for joy in small, ordinary moments.",
	"I can hold space for others while honoring my own needs.",
	"I welcome clarity, and I am patient while it arrives.",
	"I am open to seeing familiar things in a new light.",
	"I let go of what no longer fits, gently and without hurry.",
	"I bring curiosity to whatever the day offers.",
	"I am grounded, present and enough as I am.",
	"My voice matters, and I share it with care.",
	"I celebrate how far I have come.",
	"I treat myself with the same compassion I offer others.",
	"I trust my intuition to guide my next gentle step.",
	"I give myself permission to rest and begin again.",
}

// phaseMoods lists the moods that fit each lunar phase.
var phaseMoods = map[string][3]string{
	PhaseNewMoon:        {MoodHopeful, MoodReflective, MoodCalm},
	PhaseWaxingCrescent: {MoodHopeful, MoodCurious, MoodEmpowered},
	PhaseFirstQuarter:   {MoodEmpowered, MoodCurious, MoodHopeful},
	PhaseWaxingGibbous:  {MoodCurious, MoodEmpowered, MoodHopeful},
	PhaseFullMoon:       {MoodEmpowered, MoodReflective, MoodHopeful},
	PhaseWaningGibbous:  {MoodReflective, MoodCalm, MoodHopeful},
	PhaseLastQuarter:    {MoodReflective, MoodCalm, MoodCurious},
	PhaseWaningCrescent: {MoodCalm, MoodReflective, MoodHopeful},
}
