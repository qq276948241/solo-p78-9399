package main

import (
	"fmt"
	"math/rand"
)

type EffectType int

const (
	EffectBurn EffectType = iota
	EffectSlow EffectType = iota
)

type Effect struct {
	Type        EffectType
	Remaining   float64
	Value       float64
	SourceLevel int
}

type CritResult struct {
	IsCrit bool
	Damage int
}

func CritChance(level int) float64 {
	return 0.05 + float64(level-1)*0.05
}

func RollCrit(level int) bool {
	return rand.Float64() < CritChance(level)
}

func NewBurnEffect(dps float64, duration float64, sourceLevel int) Effect {
	return Effect{Type: EffectBurn, Remaining: duration, Value: dps, SourceLevel: sourceLevel}
}

func NewSlowEffect(amount float64, duration float64, sourceLevel int) Effect {
	return Effect{Type: EffectSlow, Remaining: duration, Value: amount, SourceLevel: sourceLevel}
}

func SplashBurnDPS(level int) float64 {
	return 2.0 + float64(level-1)*1.0
}

func HasEffect(e *Enemy, etype EffectType) bool {
	for _, eff := range e.Effects {
		if eff.Type == etype && eff.Remaining > 0 {
			return true
		}
	}
	return false
}

func MaxSlowAmount(e *Enemy) float64 {
	max := 0.0
	for _, eff := range e.Effects {
		if eff.Type == EffectSlow && eff.Remaining > 0 && eff.Value > max {
			max = eff.Value
		}
	}
	return max
}

func ApplyEffect(tower *Tower, target *Enemy, baseDamage int) CritResult {
	cr := CritResult{IsCrit: false, Damage: baseDamage}
	if tower.Type == TowerSniper && RollCrit(tower.Level) {
		cr.IsCrit = true
		cr.Damage = baseDamage * 2
		tower.CritCount++
		critMsg = fmt.Sprintf("暴击! %s 对目标造成 %d 伤害!", getTowerName(tower.Type), cr.Damage)
		critMsgTimer = 2.0
	}
	DealDamage(target, cr.Damage)
	if !target.Alive {
		return cr
	}
	switch tower.Type {
	case TowerSplash:
		ApplySplashBurns(tower, target)
	case TowerSlow:
		ApplySlowAndChain(tower, target)
	}
	return cr
}

func ApplySplashBurns(tower *Tower, target *Enemy) {
	tx, ty := getEnemyPos(target)
	ApplySplashAt(tower, tx, ty)
}

func ApplySplashAt(tower *Tower, x, y float64) {
	dps := SplashBurnDPS(tower.Level)
	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		ex, ey := getEnemyPos(e)
		if distance(x, y, ex, ey) <= tower.SplashRadius {
			DealDamage(e, tower.Damage)
			if e.Alive {
				AddEffect(e, NewBurnEffect(dps, 3.0, tower.Level))
			}
		}
	}
}

func ApplySlowAndChain(tower *Tower, target *Enemy) {
	AddEffect(target, NewSlowEffect(tower.SlowAmount, tower.SlowDuration, tower.Level))
	tx, ty := getEnemyPos(target)
	for _, e := range enemies {
		if !e.Alive || e == target {
			continue
		}
		ex, ey := getEnemyPos(e)
		if distance(tx, ty, ex, ey) <= 1.5 {
			AddEffect(e, NewSlowEffect(0.5, 1.0, tower.Level))
		}
	}
}

func AddEffect(e *Enemy, eff Effect) {
	if !e.Alive {
		return
	}
	for i, existing := range e.Effects {
		if existing.Type == eff.Type {
			if eff.Type == EffectBurn && eff.Value > existing.Value {
				e.Effects[i].Value = eff.Value
				e.Effects[i].Remaining = eff.Remaining
				e.Effects[i].SourceLevel = eff.SourceLevel
				return
			}
			if eff.Type == EffectSlow && eff.Remaining > existing.Remaining {
				e.Effects[i].Value = eff.Value
				e.Effects[i].Remaining = eff.Remaining
				e.Effects[i].SourceLevel = eff.SourceLevel
				return
			}
		}
	}
	e.Effects = append(e.Effects, eff)
}

func UpdateEffects(e *Enemy, dt float64) {
	i := 0
	for i < len(e.Effects) {
		eff := &e.Effects[i]
		eff.Remaining -= dt
		if eff.Remaining <= 0 {
			e.Effects = append(e.Effects[:i], e.Effects[i+1:]...)
			continue
		}
		if eff.Type == EffectBurn && e.Alive {
			ApplyBurnTick(e, eff, dt)
		}
		i++
	}
	UpdateEnemySpeed(e)
}

func ApplyBurnTick(e *Enemy, eff *Effect, dt float64) {
	burnDmg := int(eff.Value * dt)
	if burnDmg < 1 {
		if rand.Float64() < eff.Value*dt {
			burnDmg = 1
		}
	}
	if burnDmg > 0 {
		DealDamage(e, burnDmg)
	}
}

func UpdateEnemySpeed(e *Enemy) {
	slow := MaxSlowAmount(e)
	if slow > 0 {
		e.Speed = e.BaseSpeed * (1.0 - slow)
	} else {
		e.Speed = e.BaseSpeed
	}
}

func CountActiveEffects() int {
	total := 0
	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		for _, eff := range e.Effects {
			if eff.Remaining > 0 {
				total++
			}
		}
	}
	return total
}

func DealDamage(e *Enemy, dmg int) {
	if !e.Alive {
		return
	}
	actualDmg := float64(dmg) * (1.0 - e.ArmorReduction)
	e.HP -= int(actualDmg)
	if e.HP <= 0 {
		e.Alive = false
		gold += e.Reward
		totalGoldEarned += e.Reward
		kills++
	}
}
