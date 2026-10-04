package rules

// ScriptedDice returns preset rolls in order. It panics when it runs out:
// in tests that means the script is too short.
type ScriptedDice struct {
	Rolls []int
	next  int
}

func (s *ScriptedDice) D8() int {
	if s.next >= len(s.Rolls) {
		panic("scripted dice ran out")
	}
	r := s.Rolls[s.next]
	s.next++
	return r
}

// Left returns how many rolls have not been used.
func (s *ScriptedDice) Left() int { return len(s.Rolls) - s.next }
