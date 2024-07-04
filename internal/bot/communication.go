package bot

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode/utf8"
)

type triggerType int
type responseType int

const (
	svo triggerType = iota
	zov triggerType = iota

	gol          responseType = iota
	likvidirovan responseType = iota
)

type matcher struct {
	ttype triggerType
	re    *regexp.Regexp
}

var matchers = []matcher{
	{
		ttype: svo,
		re:    regexp.MustCompile("[сСsS][вВvV][оОoO]+"),
	},
	{
		ttype: zov,
		re:    regexp.MustCompile("[зЗzZ][оОoO]+[вВvV]"),
	},
}

type trigger struct {
	position int
	quote    string

	runeLength int
	ttype      triggerType
}

type response struct {
	text  string
	ttype responseType
}

func findTriggers(text string) (triggers []trigger) {
	for _, matcher := range matchers {
		matches := matcher.re.FindAllStringIndex(text, -1)
		for _, match := range matches {
			quote := text[match[0]:match[1]]
			triggers = append(triggers, trigger{
				quote:      quote,
				position:   utf8.RuneCountInString(text[:match[0]]),
				runeLength: utf8.RuneCountInString(quote),
				ttype:      matcher.ttype,
			})
		}
	}
	return triggers
}

func generateResponse() response {
	if rand.IntN(100) == 0 {
		return response{
			text:  "ЛИКВИДИРОВАН",
			ttype: likvidirovan,
		}
	}
	return response{
		text:  "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л",
		ttype: gol,
	}
}

func IsTooManyTriggers(triggerCount int, triggersLength int, textLength int) bool {
	moreTriggersThan := func(maxTriggersPerMessage int) bool {
		return triggerCount > maxTriggersPerMessage
	}
	toBigTriggersLengthTimes := func(coef float64) bool {
		return coef*float64(triggersLength) > float64(textLength)
	}

	switch {
	case 0 < textLength && textLength <= 10:
		return moreTriggersThan(1)
	case 10 < textLength && textLength <= 20:
		return moreTriggersThan(2)
	case 20 < textLength && textLength <= 30:
		return moreTriggersThan(3)
	case 30 < textLength && textLength <= 50:
		return toBigTriggersLengthTimes(2.3)
	case 50 < textLength && textLength <= 100:
		return moreTriggersThan(7) && toBigTriggersLengthTimes(3.3)
	case 100 < textLength && textLength <= 250:
		return moreTriggersThan(10) && toBigTriggersLengthTimes(5.3)
	case 250 < textLength && textLength <= 1000:
		return moreTriggersThan(15) && toBigTriggersLengthTimes(11)
	default:
		return moreTriggersThan(30) && toBigTriggersLengthTimes(25)
	}
}
