package main

import "math/rand"

type EffectType int

const (
	EffectBurn  EffectType = iota
	EffectSlow  EffectType = iota
)

type Effect struct {
	Type       EffectType
	Remaining  float64
	Value      float64
}

func newBurnEffect(dps float64, duration float64) Effect {
	return Effect{Type: EffectBurn, Remaining: duration, Value: dps}
}

func newSlowEffect(amount float64, duration float64) Effect {
	return Effect{Type: EffectSlow, Remaining: duration, Value: amount}
}

func updateEffects(e *Enemy, dt float64) {
	i := 0
	for i < len(e.Debuffs) {
		eff := &e.Debuffs[i]
		eff.Remaining -= dt
		if eff.Remaining <= 0 {
			if eff.Type == EffectSlow {
				hasOtherSlow := false
				for j, other := range e.Debuffs {
					if j != i && other.Type == EffectSlow && other.Remaining > 0 {
						hasOtherSlow = true
						break
					}
				}
				if !hasOtherSlow {
					e.Speed = e.BaseSpeed
					e.SlowTimer = 0
				}
			}
			e.Debuffs = append(e.Debuffs[:i], e.Debuffs[i+1:]...)
			continue
		}
		if eff.Type == EffectBurn && e.Alive {
			burnDmg := int(eff.Value * dt)
			if burnDmg < 1 {
				if rand.Float64() < eff.Value*dt {
					burnDmg = 1
				}
			}
			if burnDmg > 0 {
				actualDmg := float64(burnDmg) * (1.0 - e.ArmorReduction)
				e.HP -= int(actualDmg)
				if e.HP <= 0 {
					e.Alive = false
					gold += e.Reward
					totalGoldEarned += e.Reward
					kills++
				}
			}
		}
		if eff.Type == EffectSlow {
			maxSlow := 0.0
			for _, other := range e.Debuffs {
				if other.Type == EffectSlow && other.Remaining > 0 && other.Value > maxSlow {
					maxSlow = other.Value
				}
			}
			e.Speed = e.BaseSpeed * (1.0 - maxSlow)
			e.SlowTimer = eff.Remaining
		}
		i++
	}
}

func hasBurnDebuff(e *Enemy) bool {
	for _, eff := range e.Debuffs {
		if eff.Type == EffectBurn && eff.Remaining > 0 {
			return true
		}
	}
	return false
}

func countActiveDebuffs() int {
	total := 0
	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		for _, eff := range e.Debuffs {
			if eff.Remaining > 0 {
				total++
			}
		}
	}
	return total
}

func addDebuff(e *Enemy, eff Effect) {
	if !e.Alive {
		return
	}
	for i, existing := range e.Debuffs {
		if existing.Type == eff.Type {
			if eff.Type == EffectBurn && eff.Value > existing.Value {
				e.Debuffs[i].Value = eff.Value
				e.Debuffs[i].Remaining = eff.Remaining
				return
			}
			if eff.Type == EffectSlow && eff.Remaining > existing.Remaining {
				e.Debuffs[i].Value = eff.Value
				e.Debuffs[i].Remaining = eff.Remaining
				return
			}
		}
	}
	e.Debuffs = append(e.Debuffs, eff)
}

func tryCrit(tower *Tower) bool {
	critChance := 0.05 + float64(tower.Level-1)*0.05
	return rand.Float64() < critChance
}
