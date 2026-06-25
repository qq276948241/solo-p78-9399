package main

import (
	"fmt"
	"math"
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
	Value       float64 // 效果数值（燃烧=每秒伤害，减速=减速百分比）
	SourceLevel int     // 挂上时的塔等级 — 数值已按该等级算好存入 Value，每帧不再重算
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
	// dps 已按当前塔等级算好（SplashBurnDPS(sourceLevel)），直接存 Value
	return Effect{Type: EffectBurn, Remaining: duration, Value: dps, SourceLevel: sourceLevel}
}

func NewSlowEffect(amount float64, duration float64, sourceLevel int) Effect {
	// amount 已按当前塔等级算好（tower.SlowAmount 或连锁减速的 0.5），直接存 Value
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
	// 伤害计算顺序：先算暴击倍率（×2 或 ×1），再传给 DealDamage 算护甲减伤
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
	// 同类效果总是刷新到最新状态（确保塔升级后再次命中会更新数值和持续时间）
	// 效果数值已在 NewXxxEffect 时按当时塔等级算好存入 eff.Value，这里不再重算
	for i, existing := range e.Effects {
		if existing.Type == eff.Type {
			e.Effects[i].Value = eff.Value
			e.Effects[i].Remaining = eff.Remaining
			e.Effects[i].SourceLevel = eff.SourceLevel
			return
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
	// eff.Value 是 debuff 挂上时算好存进去的 dps，每帧直接使用，不重新计算
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
	if !e.Alive || dmg <= 0 {
		return
	}
	// 伤害计算顺序（重要！）：
	// 1. 先算暴击倍率（已在调用方 ApplyEffect 处理：baseDamage * 2 或 *1）
	// 2. 再算护甲减伤：伤害 * (1 - ArmorReduction)
	// 3. 四舍五入取整，最少 1 点伤害（避免小伤害被截断成 0）
	reduced := float64(dmg) * (1.0 - e.ArmorReduction)
	actualDmg := int(math.Round(reduced))
	if actualDmg < 1 {
		actualDmg = 1
	}
	e.HP -= actualDmg
	if e.HP <= 0 {
		e.Alive = false
		gold += e.Reward
		totalGoldEarned += e.Reward
		kills++
	}
}
