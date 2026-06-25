package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
)

type CellType int

const (
	CellEmpty   CellType = iota
	CellPath    CellType = iota
	CellBuild   CellType = iota
	CellStart   CellType = iota
	CellEnd     CellType = iota
)

type TowerType int

const (
	TowerSniper TowerType = iota
	TowerSplash TowerType = iota
	TowerSlow   TowerType = iota
)

type EnemyType int

const (
	EnemyNormal EnemyType = iota
	EnemyFast   EnemyType = iota
	EnemyArmor  EnemyType = iota
)

type GameState int

const (
	StateMenu     GameState = iota
	StateMapSelect GameState = iota
	StatePlaying  GameState = iota
	StateGameOver GameState = iota
)

type Point struct {
	X, Y int
}

type MapDef struct {
	Name     string
	Width    int
	Height   int
	Grid     [][]CellType
	Path     []Point
}

type Enemy struct {
	Type          EnemyType
	PathIndex     int
	Progress      float64
	HP            int
	MaxHP         int
	Speed         float64
	BaseSpeed     float64
	Reward        int
	Color         tcell.Color
	Symbol        rune
	ArmorReduction float64
	Alive         bool
	Effects       []Effect
}

type Tower struct {
	Type       TowerType
	Level      int
	X, Y       int
	Damage     int
	Range      float64
	FireRate   float64
	FireCooldown float64
	SplashRadius float64
	SlowAmount   float64
	SlowDuration float64
	TotalCost    int
	CritCount    int
}

type Projectile struct {
	X, Y     float64
	TargetX, TargetY float64
	Target   *Enemy
	Damage   int
	Speed    float64
	IsSplash bool
	SplashRadius float64
	IsSlow   bool
	SlowAmount float64
	SlowDuration float64
	Alive    bool
	SourceTower *Tower
}

type HighScore struct {
	MapName   string `json:"map_name"`
	Wave      int    `json:"wave"`
	Kills     int    `json:"kills"`
	TotalGold int    `json:"total_gold"`
	Rating    string `json:"rating"`
	Date      string `json:"date"`
}

var (
	screen      tcell.Screen
	maps        []MapDef
	currentMap  *MapDef
	state       GameState
	menuIndex   int
	mapIndex    int

	playerHP       = 100
	maxPlayerHP    = 100
	gold           = 200
	totalGoldEarned = 200
	wave           = 0
	maxWave        = 20
	waveInProgress bool
	waveTimer      float64
	waveDelay      = 8.0
	enemySpawnQueue []Enemy
	enemySpawnTimer float64
	enemies         []*Enemy
	towers          []*Tower
	projectiles     []*Projectile
	cursorX, cursorY int
	selectedTowerType TowerType
	kills           = 0
	selectedTower   *Tower
	lastTime        time.Time
	highScores      []HighScore
	scoreFile       = "highscores.json"
	critMsg         string
	critMsgTimer    float64
)

func initMaps() {
	maps = make([]MapDef, 2)

	maps[0] = createMap1()
	maps[1] = createMap2()
}

func createMap1() MapDef {
	w, h := 30, 15
	grid := make([][]CellType, h)
	for y := range grid {
		grid[y] = make([]CellType, w)
		for x := range grid[y] {
			grid[y][x] = CellBuild
		}
	}

	path := []Point{}
	addPath := func(x, y int) {
		grid[y][x] = CellPath
		path = append(path, Point{X: x, Y: y})
	}

	addPath(0, 7)
	for x := 1; x <= 6; x++ {
		addPath(x, 7)
	}
	for y := 6; y >= 3; y-- {
		addPath(6, y)
	}
	for x := 7; x <= 14; x++ {
		addPath(x, 3)
	}
	for y := 4; y <= 11; y++ {
		addPath(14, y)
	}
	for x := 13; x >= 8; x-- {
		addPath(x, 11)
	}
	for y := 10; y >= 7; y-- {
		addPath(8, y)
	}
	for x := 9; x <= 22; x++ {
		addPath(x, 7)
	}
	for y := 6; y >= 2; y-- {
		addPath(22, y)
	}
	for x := 23; x <= 29; x++ {
		addPath(x, 2)
	}

	grid[path[0].Y][path[0].X] = CellStart
	grid[path[len(path)-1].Y][path[len(path)-1].X] = CellEnd

	return MapDef{
		Name:   "蜿蜒之路",
		Width:  w,
		Height: h,
		Grid:   grid,
		Path:   path,
	}
}

func createMap2() MapDef {
	w, h := 30, 15
	grid := make([][]CellType, h)
	for y := range grid {
		grid[y] = make([]CellType, w)
		for x := range grid[y] {
			grid[y][x] = CellBuild
		}
	}

	path := []Point{}
	addPath := func(x, y int) {
		grid[y][x] = CellPath
		path = append(path, Point{X: x, Y: y})
	}

	for x := 0; x <= 8; x++ {
		addPath(x, 2)
	}
	for y := 3; y <= 6; y++ {
		addPath(8, y)
	}
	for x := 7; x >= 3; x-- {
		addPath(x, 6)
	}
	for y := 7; y <= 12; y++ {
		addPath(3, y)
	}
	for x := 4; x <= 15; x++ {
		addPath(x, 12)
	}
	for y := 11; y >= 5; y-- {
		addPath(15, y)
	}
	for x := 16; x <= 25; x++ {
		addPath(x, 5)
	}
	for y := 6; y <= 9; y++ {
		addPath(25, y)
	}
	for x := 26; x <= 29; x++ {
		addPath(x, 9)
	}

	grid[path[0].Y][path[0].X] = CellStart
	grid[path[len(path)-1].Y][path[len(path)-1].X] = CellEnd

	return MapDef{
		Name:   "之字形峡谷",
		Width:  w,
		Height: h,
		Grid:   grid,
		Path:   path,
	}
}

func getTowerStats(t TowerType, level int) (damage int, towerRange float64, fireRate float64, splashRadius float64, slowAmount float64, slowDuration float64, cost int) {
	switch t {
	case TowerSniper:
		baseDmg := [4]int{0, 40, 80, 160}
		baseRange := [4]float64{0, 5.0, 6.0, 7.5}
		baseRate := [4]float64{0, 2.0, 2.5, 3.0}
		baseCost := [4]int{0, 80, 120, 200}
		damage = baseDmg[level]
		towerRange = baseRange[level]
		fireRate = baseRate[level]
		cost = baseCost[level]
	case TowerSplash:
		baseDmg := [4]int{0, 20, 35, 60}
		baseRange := [4]float64{0, 3.5, 4.0, 4.5}
		baseRate := [4]float64{0, 1.2, 1.5, 1.8}
		baseSplash := [4]float64{0, 1.5, 1.8, 2.2}
		baseCost := [4]int{0, 100, 150, 250}
		damage = baseDmg[level]
		towerRange = baseRange[level]
		fireRate = baseRate[level]
		splashRadius = baseSplash[level]
		cost = baseCost[level]
	case TowerSlow:
		baseDmg := [4]int{0, 8, 15, 25}
		baseRange := [4]float64{0, 3.0, 3.5, 4.0}
		baseRate := [4]float64{0, 1.5, 1.8, 2.2}
		baseSlowAmount := [4]float64{0, 0.5, 0.55, 0.6}
		baseSlowDur := [4]float64{0, 2.0, 2.5, 3.0}
		baseCost := [4]int{0, 60, 90, 150}
		damage = baseDmg[level]
		towerRange = baseRange[level]
		fireRate = baseRate[level]
		slowAmount = baseSlowAmount[level]
		slowDuration = baseSlowDur[level]
		cost = baseCost[level]
	}
	return
}

func getTowerName(t TowerType) string {
	switch t {
	case TowerSniper:
		return "狙击塔"
	case TowerSplash:
		return "溅射塔"
	case TowerSlow:
		return "减速塔"
	}
	return ""
}

func getTowerSymbol(t TowerType) rune {
	switch t {
	case TowerSniper:
		return 'S'
	case TowerSplash:
		return 'P'
	case TowerSlow:
		return 'D'
	}
	return '?'
}

func getTowerColor(t TowerType) tcell.Color {
	switch t {
	case TowerSniper:
		return tcell.ColorBlue
	case TowerSplash:
		return tcell.ColorYellow
	case TowerSlow:
		return tcell.ColorGreen
	}
	return tcell.ColorWhite
}

func createEnemy(etype EnemyType, waveNum int) Enemy {
	waveMult := 1.0 + float64(waveNum-1)*0.15
	e := Enemy{Type: etype, PathIndex: 0, Progress: 0, Alive: true}

	switch etype {
	case EnemyNormal:
		e.MaxHP = int(float64(50) * waveMult)
		e.BaseSpeed = 2.0
		e.Color = tcell.ColorGreen
		e.Symbol = 'g'
		e.Reward = 10 + waveNum
		e.ArmorReduction = 0
	case EnemyFast:
		e.MaxHP = int(float64(30) * waveMult)
		e.BaseSpeed = 3.5
		e.Color = tcell.ColorYellow
		e.Symbol = 'f'
		e.Reward = 8 + waveNum
		e.ArmorReduction = 0
	case EnemyArmor:
		e.MaxHP = int(float64(180) * waveMult)
		e.BaseSpeed = 1.2
		e.Color = tcell.ColorRed
		e.Symbol = 'r'
		e.Reward = 25 + waveNum*2
		e.ArmorReduction = 0.4
	}
	e.HP = e.MaxHP
	e.Speed = e.BaseSpeed
	return e
}

func generateWaveEnemies(waveNum int) []Enemy {
	queue := []Enemy{}
	normalCount := 5 + waveNum*2
	fastCount := 0
	armorCount := 0

	if waveNum >= 4 {
		fastCount = waveNum
	}
	if waveNum >= 10 {
		armorCount = (waveNum - 8)
	}
	if waveNum >= 17 {
		armorCount += 3
		normalCount += 5
	}

	order := make([]EnemyType, 0)
	for i := 0; i < normalCount; i++ {
		order = append(order, EnemyNormal)
	}
	for i := 0; i < fastCount; i++ {
		order = append(order, EnemyFast)
	}
	for i := 0; i < armorCount; i++ {
		order = append(order, EnemyArmor)
	}

	for i := len(order) - 1; i > 0; i-- {
		j := i % (i + 1)
		if j == 0 && i > 0 {
			j = i - 1
		}
		order[i], order[j] = order[j], order[i]
	}

	for _, et := range order {
		queue = append(queue, createEnemy(et, waveNum))
	}
	return queue
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1))
}

func getEnemyPos(e *Enemy) (float64, float64) {
	if e.PathIndex >= len(currentMap.Path)-1 {
		last := currentMap.Path[len(currentMap.Path)-1]
		return float64(last.X), float64(last.Y)
	}
	cur := currentMap.Path[e.PathIndex]
	next := currentMap.Path[e.PathIndex+1]
	return float64(cur.X) + (float64(next.X)-float64(cur.X))*e.Progress,
		float64(cur.Y) + (float64(next.Y)-float64(cur.Y))*e.Progress
}

func startGame(mapIdx int) {
	mapIndex = mapIdx
	currentMap = &maps[mapIdx]
	playerHP = maxPlayerHP
	gold = 200
	totalGoldEarned = 200
	wave = 0
	maxWave = 20
	waveInProgress = false
	waveTimer = 3.0
	enemySpawnQueue = nil
	enemySpawnTimer = 0
	enemies = nil
	towers = nil
	projectiles = nil
	kills = 0
	selectedTower = nil
	cursorX = 1
	cursorY = 1
	selectedTowerType = TowerSniper
	lastTime = time.Now()
	critMsg = ""
	critMsgTimer = 0
	state = StatePlaying
}

func getTowerAt(x, y int) *Tower {
	for _, t := range towers {
		if t.X == x && t.Y == y {
			return t
		}
	}
	return nil
}

func buildTower(x, y int, ttype TowerType) bool {
	if currentMap.Grid[y][x] != CellBuild {
		return false
	}
	if getTowerAt(x, y) != nil {
		return false
	}
	dmg, rng, rate, splash, slowAmt, slowDur, cost := getTowerStats(ttype, 1)
	if gold < cost {
		return false
	}
	gold -= cost
	towers = append(towers, &Tower{
		Type: ttype, Level: 1, X: x, Y: y,
		Damage: dmg, Range: rng, FireRate: rate,
		SplashRadius: splash, SlowAmount: slowAmt, SlowDuration: slowDur,
		TotalCost: cost,
	})
	return true
}

func upgradeTower(t *Tower) bool {
	if t.Level >= 3 {
		return false
	}
	_, _, _, _, _, _, cost := getTowerStats(t.Type, t.Level+1)
	if gold < cost {
		return false
	}
	gold -= cost
	t.Level++
	dmg, rng, rate, splash, slowAmt, slowDur, _ := getTowerStats(t.Type, t.Level)
	t.Damage = dmg
	t.Range = rng
	t.FireRate = rate
	t.SplashRadius = splash
	t.SlowAmount = slowAmt
	t.SlowDuration = slowDur
	t.TotalCost += cost
	return true
}

func sellTower(t *Tower) {
	refund := t.TotalCost / 2
	gold += refund
	for i, tw := range towers {
		if tw == t {
			towers = append(towers[:i], towers[i+1:]...)
			break
		}
	}
	selectedTower = nil
}

func fireTower(t *Tower, target *Enemy) {
	tx, ty := float64(t.X), float64(t.Y)
	ex, ey := getEnemyPos(target)

	proj := &Projectile{
		X: tx, Y: ty, TargetX: ex, TargetY: ey,
		Target: target, Damage: t.Damage, Speed: 15.0,
		IsSplash: t.Type == TowerSplash,
		SplashRadius: t.SplashRadius,
		IsSlow: t.Type == TowerSlow,
		SlowAmount: t.SlowAmount, SlowDuration: t.SlowDuration,
		Alive: true,
		SourceTower: t,
	}
	projectiles = append(projectiles, proj)
}

func findTargetForTower(t *Tower) *Enemy {
	tx, ty := float64(t.X), float64(t.Y)
	var best *Enemy
	bestProgress := -1.0
	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		ex, ey := getEnemyPos(e)
		if distance(tx, ty, ex, ey) <= t.Range {
			prog := float64(e.PathIndex) + e.Progress
			if prog > bestProgress {
				bestProgress = prog
				best = e
			}
		}
	}
	return best
}

func moveEnemy(e *Enemy, dt float64) {
	moveAmt := e.Speed * dt
	for moveAmt > 0 && e.Alive {
		if e.PathIndex >= len(currentMap.Path)-1 {
			playerHP -= 10
			e.Alive = false
			if playerHP <= 0 {
				endGame(false)
			}
			break
		}
		if moveAmt >= 1.0-e.Progress {
			moveAmt -= (1.0 - e.Progress)
			e.PathIndex++
			e.Progress = 0
		} else {
			e.Progress += moveAmt
			moveAmt = 0
		}
	}
}

func resolveProjectileHit(p *Projectile) {
	if p.SourceTower == nil {
		return
	}
	if p.IsSplash {
		ApplySplashAt(p.SourceTower, p.TargetX, p.TargetY)
		return
	}
	if p.Target == nil || !p.Target.Alive {
		return
	}
	ApplyEffect(p.SourceTower, p.Target, p.Damage)
}

func update(dt float64) {
	if !waveInProgress {
		waveTimer -= dt
		if waveTimer <= 0 && wave < maxWave {
			wave++
			waveInProgress = true
			enemySpawnQueue = generateWaveEnemies(wave)
			enemySpawnTimer = 0
		}
	} else {
		enemySpawnTimer -= dt
		if enemySpawnTimer <= 0 && len(enemySpawnQueue) > 0 {
			enemy := enemySpawnQueue[0]
			enemySpawnQueue = enemySpawnQueue[1:]
			enemies = append(enemies, &enemy)
			enemySpawnTimer = 0.6
		}
		if len(enemySpawnQueue) == 0 {
			allDeadOrDone := true
			for _, e := range enemies {
				if e.Alive {
					allDeadOrDone = false
					break
				}
			}
			if allDeadOrDone {
				waveInProgress = false
				if wave >= maxWave {
					endGame(true)
					return
				}
				waveTimer = waveDelay
			}
		}
	}

	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		UpdateEffects(e, dt)
		moveEnemy(e, dt)
	}

	for _, t := range towers {
		t.FireCooldown -= dt
		if t.FireCooldown <= 0 {
			target := findTargetForTower(t)
			if target != nil {
				fireTower(t, target)
				t.FireCooldown = 1.0 / t.FireRate
			}
		}
	}

	for _, p := range projectiles {
		if !p.Alive {
			continue
		}
		if p.Target != nil && p.Target.Alive {
			p.TargetX, p.TargetY = getEnemyPos(p.Target)
		}
		dx := p.TargetX - p.X
		dy := p.TargetY - p.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		moveAmt := p.Speed * dt
		if dist <= moveAmt {
			resolveProjectileHit(p)
			p.Alive = false
		} else {
			p.X += (dx / dist) * moveAmt
			p.Y += (dy / dist) * moveAmt
		}
	}

	if critMsgTimer > 0 {
		critMsgTimer -= dt
		if critMsgTimer <= 0 {
			critMsg = ""
		}
	}

	enemies = filterAliveEnemies(enemies)
	projectiles = filterAliveProjectiles(projectiles)
}

func filterAliveEnemies(arr []*Enemy) []*Enemy {
	res := make([]*Enemy, 0, len(arr))
	for _, x := range arr {
		if x.Alive {
			res = append(res, x)
		}
	}
	return res
}

func filterAliveProjectiles(arr []*Projectile) []*Projectile {
	res := make([]*Projectile, 0, len(arr))
	for _, x := range arr {
		if x.Alive {
			res = append(res, x)
		}
	}
	return res
}

func endGame(won bool) {
	state = StateGameOver
	calcRating := func() string {
		hpRatio := float64(playerHP) / float64(maxPlayerHP)
		waveRatio := float64(wave) / float64(maxWave)
		score := hpRatio*0.5 + waveRatio*0.3 + float64(kills)/500.0*0.2
		if won && hpRatio > 0.7 {
			return "S"
		} else if score > 0.75 {
			return "A"
		} else if score > 0.5 {
			return "B"
		}
		return "C"
	}
	rating := calcRating()
	hs := HighScore{
		MapName:   currentMap.Name,
		Wave:      wave,
		Kills:     kills,
		TotalGold: totalGoldEarned,
		Rating:    rating,
		Date:      time.Now().Format("2006-01-02 15:04"),
	}
	highScores = append(highScores, hs)
	sort.Slice(highScores, func(i, j int) bool {
		if highScores[i].Wave != highScores[j].Wave {
			return highScores[i].Wave > highScores[j].Wave
		}
		return highScores[i].Kills > highScores[j].Kills
	})
	if len(highScores) > 10 {
		highScores = highScores[:10]
	}
	saveHighScores()
}

func saveHighScores() {
	data, err := json.MarshalIndent(highScores, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(scoreFile, data, 0644)
}

func loadHighScores() {
	data, err := os.ReadFile(scoreFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &highScores)
}

func drawText(x, y int, s string, fg tcell.Color, bg tcell.Color) {
	for i, c := range s {
		screen.SetContent(x+i, y, c, nil, tcell.StyleDefault.Foreground(fg).Background(bg))
	}
}

func drawTextCenter(y int, s string, fg tcell.Color, bg tcell.Color, width int) {
	x := (width - len(s)) / 2
	drawText(x, y, s, fg, bg)
}

func drawHUD(offsetX, offsetY, screenW int) {
	bar := tcell.ColorBlue
	waveStr := fmt.Sprintf(" 波次: %d/%d ", wave, maxWave)
	hpStr := fmt.Sprintf(" 血量: %d/%d ", playerHP, maxPlayerHP)
	goldStr := fmt.Sprintf(" 金币: %d ", gold)
	var nextWaveStr string
	if !waveInProgress && wave < maxWave {
		nextWaveStr = fmt.Sprintf(" 下波: %.0fs ", waveTimer)
	} else if waveInProgress {
		nextWaveStr = fmt.Sprintf(" 进行中 剩余:%d+%d ", len(enemySpawnQueue), len(enemies))
	} else {
		nextWaveStr = " 完成! "
	}
	towerStr := fmt.Sprintf(" 选中塔: %s(%s) ", getTowerName(selectedTowerType), string(getTowerSymbol(selectedTowerType)))
	keysStr := " [1/2/3]切塔 [回车]建造 [Q]升级 [T]出售 [ESC]菜单 "

	x := offsetX
	drawText(x, offsetY, waveStr, tcell.ColorWhite, bar)
	x += len(waveStr)
	hpColor := tcell.ColorWhite
	if playerHP <= 30 {
		hpColor = tcell.ColorRed
	}
	drawText(x, offsetY, hpStr, hpColor, bar)
	x += len(hpStr)
	drawText(x, offsetY, goldStr, tcell.ColorYellow, bar)
	x += len(goldStr)
	drawText(x, offsetY, nextWaveStr, tcell.ColorGreen, bar)
	x += len(nextWaveStr)
	for ; x < offsetX+screenW; x++ {
		screen.SetContent(x, offsetY, ' ', nil, tcell.StyleDefault.Background(bar))
	}

	y := offsetY + 1
	drawText(offsetX, y, towerStr, getTowerColor(selectedTowerType), tcell.ColorBlack)
	infoW := len(towerStr)
	_, _, _, _, _, _, cost1 := getTowerStats(selectedTowerType, 1)
	info := fmt.Sprintf(" Lv1:%dg ", cost1)
	if gold < cost1 {
		drawText(offsetX+infoW, y, info, tcell.ColorRed, tcell.ColorBlack)
	} else {
		drawText(offsetX+infoW, y, info, tcell.ColorGray, tcell.ColorBlack)
	}
	infoW += len(info)

	for x := offsetX + infoW; x < offsetX+screenW; x++ {
		screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(tcell.ColorBlack))
	}

	y2 := offsetY + 2
	keyColor := tcell.ColorGray
	for i := 0; i < screenW; i++ {
		screen.SetContent(offsetX+i, y2, ' ', nil, tcell.StyleDefault.Background(tcell.ColorBlack))
	}
	drawText(offsetX, y2, keysStr, keyColor, tcell.ColorBlack)

	y3 := offsetY + 3
	debuffTotal := CountActiveEffects()
	effectStr := fmt.Sprintf(" 场上Debuff: %d ", debuffTotal)
	if selectedTower != nil && selectedTower.Type == TowerSniper {
		effectStr += fmt.Sprintf(" | %s暴击次数: %d ", getTowerName(selectedTower.Type), selectedTower.CritCount)
		critChance := CritChance(selectedTower.Level)
		effectStr += fmt.Sprintf("暴击率: %.0f%% ", critChance*100)
	}
	if selectedTower != nil && selectedTower.Type == TowerSplash {
		burnDps := int(SplashBurnDPS(selectedTower.Level))
		effectStr += fmt.Sprintf(" | %s燃烧: %d/s持续3秒 ", getTowerName(selectedTower.Type), burnDps)
	}
	if selectedTower != nil && selectedTower.Type == TowerSlow {
		effectStr += fmt.Sprintf(" | %s连锁减速: 周围1格50%%1秒 ", getTowerName(selectedTower.Type))
	}
	for i := 0; i < screenW; i++ {
		screen.SetContent(offsetX+i, y3, ' ', nil, tcell.StyleDefault.Background(tcell.ColorBlack))
	}
	drawText(offsetX, y3, effectStr, tcell.ColorWhite, tcell.ColorBlack)
}

func drawGame() {
	screenW, screenH := screen.Size()
	mapOffsetX := (screenW - currentMap.Width) / 2
	mapOffsetY := (screenH - currentMap.Height) / 2
	if mapOffsetY < 7 {
		mapOffsetY = 7
	}

	drawHUD(0, 1, screenW)

	for y := 0; y < currentMap.Height; y++ {
		for x := 0; x < currentMap.Width; x++ {
			absX := mapOffsetX + x
			absY := mapOffsetY + y
			cell := currentMap.Grid[y][x]
			style := tcell.StyleDefault
			var ch rune
			switch cell {
			case CellPath:
				ch = '.'
				style = style.Foreground(tcell.ColorGray)
			case CellBuild:
				ch = '#'
				style = style.Foreground(tcell.ColorGray)
			case CellStart:
				ch = 'S'
				style = style.Foreground(tcell.ColorGreen).Bold(true)
			case CellEnd:
				ch = 'E'
				style = style.Foreground(tcell.ColorRed).Bold(true)
			}
			tw := getTowerAt(x, y)
			if tw != nil {
				ch = getTowerSymbol(tw.Type)
				style = style.Foreground(getTowerColor(tw.Type)).Bold(true)
				if tw.Level > 1 {
					style = style.Background(tcell.ColorGray)
				}
				if tw.Level == 3 {
					style = style.Background(tcell.ColorGreen)
				}
			}
			screen.SetContent(absX, absY, ch, nil, style)
		}
	}

	for _, t := range towers {
		if selectedTower == t {
			r := int(t.Range)
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					if dx*dx+dy*dy <= r*r {
						ax := mapOffsetX + t.X + dx
						ay := mapOffsetY + t.Y + dy
						if ax >= 0 && ax < screenW && ay >= 0 && ay < screenH {
							_, _, existingStyle, _ := screen.GetContent(ax, ay)
							screen.SetContent(ax, ay, '·', nil, existingStyle.Foreground(tcell.ColorGray))
						}
					}
				}
			}
		}
	}

	for _, e := range enemies {
		if !e.Alive {
			continue
		}
		ex, ey := getEnemyPos(e)
		exI := int(math.Round(ex))
		eyI := int(math.Round(ey))
		absX := mapOffsetX + exI
		absY := mapOffsetY + eyI
		if absX >= 0 && absX < screenW && absY >= 0 && absY < screenH {
			color := e.Color
			style := tcell.StyleDefault.Foreground(color).Bold(true)
			burning := HasEffect(e, EffectBurn)
			hasSlow := HasEffect(e, EffectSlow)
			if hasSlow && burning {
				style = style.Background(tcell.ColorRed)
			} else if hasSlow {
				style = style.Background(tcell.ColorBlue)
			} else if burning {
				style = style.Background(tcell.ColorRed)
			}
			screen.SetContent(absX, absY, e.Symbol, nil, style)

			hpRatio := float64(e.HP) / float64(e.MaxHP)
			barX := absX - 1
			barY := absY - 1
			if barX >= 0 && barX+2 < screenW && barY >= 0 {
				filled := int(hpRatio * 3)
				for i := 0; i < 3; i++ {
					ch := '░'
					c := tcell.ColorGray
					if i < filled {
						ch = '█'
						if hpRatio > 0.6 {
							c = tcell.ColorGreen
						} else if hpRatio > 0.3 {
							c = tcell.ColorYellow
						} else {
							c = tcell.ColorRed
						}
					}
					screen.SetContent(barX+i, barY, ch, nil, tcell.StyleDefault.Foreground(c))
				}
			}
		}
	}

	for _, p := range projectiles {
		if !p.Alive {
			continue
		}
		absX := mapOffsetX + int(math.Round(p.X))
		absY := mapOffsetY + int(math.Round(p.Y))
		if absX >= 0 && absX < screenW && absY >= 0 && absY < screenH {
			color := tcell.ColorWhite
			if p.IsSplash {
				color = tcell.ColorYellow
			} else if p.IsSlow {
				color = tcell.ColorBlue
			}
			screen.SetContent(absX, absY, '*', nil, tcell.StyleDefault.Foreground(color).Bold(true))
		}
	}

	cell := currentMap.Grid[cursorY][cursorX]
	cursorStyle := tcell.StyleDefault.Reverse(true).Bold(true)
	tw := getTowerAt(cursorX, cursorY)
	var ch rune
	if tw != nil {
		ch = getTowerSymbol(tw.Type)
		cursorStyle = cursorStyle.Foreground(getTowerColor(tw.Type))
		selectedTower = tw
	} else {
		switch cell {
		case CellPath, CellStart, CellEnd:
			ch = 'X'
			cursorStyle = cursorStyle.Foreground(tcell.ColorGray)
		case CellBuild:
			ch = getTowerSymbol(selectedTowerType)
			cursorStyle = cursorStyle.Foreground(getTowerColor(selectedTowerType))
		}
		selectedTower = nil
	}
	screen.SetContent(mapOffsetX+cursorX, mapOffsetY+cursorY, ch, nil, cursorStyle)

	if selectedTower != nil {
		infoY := mapOffsetY + currentMap.Height + 1
		_, _, _, _, _, _, nextCost := getTowerStats(selectedTower.Type, selectedTower.Level+1)
		info := fmt.Sprintf("  %s Lv%d | 伤害:%d 射程:%.1f 射速:%.1f 总投资:%dg | ",
			getTowerName(selectedTower.Type), selectedTower.Level,
			selectedTower.Damage, selectedTower.Range, selectedTower.FireRate, selectedTower.TotalCost)
		if selectedTower.Level < 3 {
			info += fmt.Sprintf("升级需%dg 出售返还%dg [Q升级] [T出售] ", nextCost, selectedTower.TotalCost/2)
		} else {
			info += fmt.Sprintf("已满级 出售返还%dg [T出售] ", selectedTower.TotalCost/2)
		}
		x := (screenW - len(info)) / 2
		if x < 0 {
			x = 0
		}
		drawText(x, infoY, info, tcell.ColorWhite, tcell.ColorRed)
	}

	if critMsgTimer > 0 && critMsg != "" {
		critY := mapOffsetY + currentMap.Height + 2
		if selectedTower != nil {
			critY++
		}
		x := (screenW - len(critMsg)) / 2
		if x < 0 {
			x = 0
		}
		drawText(x, critY, critMsg, tcell.ColorYellow, tcell.ColorRed)
	}
}

func drawMenu() {
	screenW, screenH := screen.Size()
	bg := tcell.ColorBlack
	for y := 0; y < screenH; y++ {
		for x := 0; x < screenW; x++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(bg))
		}
	}

	title := "╔══════════════════════════════════════════════════════════════╗"
	title2 := "║                    终 端 塔 防 游 戏                        ║"
	title3 := "╚══════════════════════════════════════════════════════════════╝"
	centerY := screenH / 4
	drawTextCenter(centerY, title, tcell.ColorBlue, bg, screenW)
	drawTextCenter(centerY+1, title2, tcell.ColorYellow, bg, screenW)
	drawTextCenter(centerY+2, title3, tcell.ColorBlue, bg, screenW)

	items := []struct {
		text  string
		color tcell.Color
	}{
		{"1. 开始游戏 - 选择地图", tcell.ColorGreen},
		{"2. 历史最高分榜", tcell.ColorYellow},
		{"3. 退出游戏", tcell.ColorRed},
	}

	selBG := tcell.ColorBlue
	itemY := centerY + 5
	for i, item := range items {
		prefix := "   "
		b := bg
		c := item.color
		if i == menuIndex {
			prefix = " ► "
			b = selBG
			c = tcell.ColorWhite
		}
		line := prefix + item.text + "    "
		drawTextCenter(itemY+i, line, c, b, screenW)
	}

	tips := []string{
		"",
		"操作说明： ↑↓方向键选择菜单 回车确认",
		"游戏中：方向键移动光标 1/2/3切换塔类型 回车建造 Q升级 T出售 ESC返回菜单",
		"三种塔：S=狙击(高伤单体) P=溅射(范围伤害) D=减速(减速50%+伤害)",
		"敌人颜色：g=普通(绿) f=快速(黄) r=重甲(红减伤40%)",
	}
	tipY := itemY + 6
	for i, tip := range tips {
		drawTextCenter(tipY+i, tip, tcell.ColorGray, bg, screenW)
	}

	if len(highScores) > 0 {
		scoreTitle := "── 历史前三名 ──"
		drawTextCenter(screenH-7, scoreTitle, tcell.ColorYellow, bg, screenW)
		top := 3
		if len(highScores) < 3 {
			top = len(highScores)
		}
		for i := 0; i < top; i++ {
			hs := highScores[i]
			line := fmt.Sprintf("%d. %s 波次:%d 击杀:%d 金币:%d 评级:%s [%s]",
				i+1, hs.MapName, hs.Wave, hs.Kills, hs.TotalGold, hs.Rating, hs.Date)
			colors := []tcell.Color{tcell.ColorYellow, tcell.ColorWhite, tcell.ColorRed}
			drawTextCenter(screenH-6+i, line, colors[i], bg, screenW)
		}
	}
}

func drawMapSelect() {
	screenW, screenH := screen.Size()
	bg := tcell.ColorBlack
	for y := 0; y < screenH; y++ {
		for x := 0; x < screenW; x++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(bg))
		}
	}

	title := "=== 选择地图 ==="
	drawTextCenter(screenH/4, title, tcell.ColorYellow, bg, screenW)

	startY := screenH/4 + 3
	cardW := 40
	cardH := 10
	totalW := len(maps)*cardW + (len(maps)-1)*3
	startX := (screenW - totalW) / 2

	for i, m := range maps {
		x := startX + i*(cardW+3)
		y := startY
		isSel := i == mapIndex
		border := tcell.ColorGray
		titleC := tcell.ColorWhite
		bgCard := tcell.ColorBlue
		if isSel {
			border = tcell.ColorGreen
			titleC = tcell.ColorYellow
			bgCard = tcell.ColorBlue
		}

		for cy := 0; cy < cardH; cy++ {
			for cx := 0; cx < cardW; cx++ {
				ch := ' '
				style := tcell.StyleDefault.Background(bgCard)
				if cy == 0 || cy == cardH-1 {
					ch = '═'
					style = style.Foreground(border)
				} else if cx == 0 || cx == cardW-1 {
					ch = '║'
					style = style.Foreground(border)
				}
				if cy == 0 && cx == 0 {
					ch = '╔'
				} else if cy == 0 && cx == cardW-1 {
					ch = '╗'
				} else if cy == cardH-1 && cx == 0 {
					ch = '╚'
				} else if cy == cardH-1 && cx == cardW-1 {
					ch = '╝'
				}
				screen.SetContent(x+cx, y+cy, ch, nil, style)
			}
		}

		mapTitle := fmt.Sprintf(" 地图 %d: %s ", i+1, m.Name)
		drawText(x+(cardW-len(mapTitle))/2, y+1, mapTitle, titleC, bgCard)

		miniScale := 2
		miniH := (m.Height + miniScale - 1) / miniScale
		miniW := (m.Width + miniScale - 1) / miniScale
		startMY := y + 3
		startMX := x + (cardW-miniW)/2
		for my := 0; my < miniH; my++ {
			for mx := 0; mx < miniW; mx++ {
				realX := mx * miniScale
				realY := my * miniScale
				if realX >= m.Width || realY >= m.Height {
					continue
				}
				cell := m.Grid[realY][realX]
				ch := ' '
				c := tcell.ColorGray
				switch cell {
				case CellPath:
					ch = '·'
					c = tcell.ColorGray
				case CellBuild:
					ch = '▒'
					c = tcell.ColorGray
				case CellStart:
					ch = 'S'
					c = tcell.ColorGreen
				case CellEnd:
					ch = 'E'
					c = tcell.ColorRed
				}
				screen.SetContent(startMX+mx, startMY+my, ch, nil,
					tcell.StyleDefault.Background(bgCard).Foreground(c))
			}
		}

		info1 := fmt.Sprintf(" 尺寸: %dx%d 路径点: %d ", m.Width, m.Height, len(m.Path))
		drawText(x+(cardW-len(info1))/2, y+cardH-3, info1, tcell.ColorWhite, bgCard)

		if isSel {
			sel := " ◄ 选中 ► "
			drawText(x+(cardW-len(sel))/2, y+cardH-2, sel, tcell.ColorGreen, bgCard)
		}
	}

	nav := "← →方向键选择地图   回车开始游戏   ESC返回主菜单"
	drawTextCenter(startY+cardH+3, nav, tcell.ColorBlue, bg, screenW)
}

func drawGameOver() {
	screenW, screenH := screen.Size()
	bg := tcell.ColorRed
	won := playerHP > 0 && wave >= maxWave
	if won {
		bg = tcell.ColorGreen
	}
	for y := 0; y < screenH; y++ {
		for x := 0; x < screenW; x++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(bg))
		}
	}

	centerY := screenH / 4
	var title string
	titleColor := tcell.ColorRed
	if won {
		title = "╔════════════════════════════════════╗  胜 利 !  ╔════════════════════════════════════╗"
		titleColor = tcell.ColorGreen
	} else {
		title = "╔════════════════════════════════════╗ 游 戏 结 束 ╔════════════════════════════════════╗"
	}
	drawTextCenter(centerY, title, titleColor, bg, screenW)

	wonStr := "你成功守住了基地！"
	if !won {
		wonStr = "你的基地被攻破了..."
	}
	drawTextCenter(centerY+2, wonStr, tcell.ColorWhite, bg, screenW)

	rating := "C"
	if len(highScores) > 0 {
		rating = highScores[0].Rating
	}
	stats := []struct {
		key, val string
		c        tcell.Color
	}{
		{"坚持波次", fmt.Sprintf("%d / %d", wave, maxWave), tcell.ColorBlue},
		{"击杀敌人", fmt.Sprintf("%d", kills), tcell.ColorYellow},
		{"总金币收入", fmt.Sprintf("%d", totalGoldEarned), tcell.ColorYellow},
		{"剩余血量", fmt.Sprintf("%d / %d", playerHP, maxPlayerHP), tcell.ColorGreen},
		{"本局评级", rating, tcell.ColorRed},
		{"使用地图", currentMap.Name, tcell.ColorWhite},
	}

	tableY := centerY + 5
	maxW := 30
	for i, s := range stats {
		line := fmt.Sprintf("  %-12s: %-14s  ", s.key, s.val)
		if len(line) < maxW {
			for j := len(line); j < maxW; j++ {
				line += " "
			}
		}
		frame := "│" + line + "│"
		if i == 4 {
			drawTextCenter(tableY+i, frame, tcell.ColorWhite, tcell.ColorRed, screenW)
		} else {
			drawTextCenter(tableY+i, frame, s.c, tcell.ColorBlack, screenW)
		}
	}

	scoreDesc := map[string]string{
		"S": "完美通关！你是塔防大师！",
		"A": "表现出色！继续保持！",
		"B": "稳扎稳打，不错的发挥！",
		"C": "还需要努力，再来一局吧！",
	}
	if desc, ok := scoreDesc[rating]; ok {
		drawTextCenter(tableY+len(stats)+1, "── "+desc+" ──", tcell.ColorYellow, bg, screenW)
	}

	menuY := tableY + len(stats) + 4
	items := []struct {
		text  string
		color tcell.Color
		index int
	}{
		{"[1] 再来一局 (同地图)", tcell.ColorGreen, 0},
		{"[2] 换地图重开", tcell.ColorBlue, 1},
		{"[3] 返回主菜单", tcell.ColorWhite, 2},
	}
	for i, item := range items {
		drawTextCenter(menuY+i, item.text, item.color, bg, screenW)
	}

	if len(highScores) > 0 {
		stTitle := "── 历史排行榜 ──"
		drawTextCenter(screenH-7, stTitle, tcell.ColorYellow, bg, screenW)
		top := 3
		if len(highScores) < top {
			top = len(highScores)
		}
		for i := 0; i < top; i++ {
			hs := highScores[i]
			line := fmt.Sprintf("No.%d  %s  波次:%d  击杀:%d  金币:%d  评级:%s",
				i+1, hs.MapName, hs.Wave, hs.Kills, hs.TotalGold, hs.Rating)
			colors := []tcell.Color{tcell.ColorYellow, tcell.ColorWhite, tcell.ColorRed}
			drawTextCenter(screenH-6+i, line, colors[i], bg, screenW)
		}
	}
}

func handleKey(ev *tcell.EventKey) {
	switch state {
	case StateMenu:
		switch ev.Key() {
		case tcell.KeyUp:
			if menuIndex > 0 {
				menuIndex--
			}
		case tcell.KeyDown:
			if menuIndex < 2 {
				menuIndex++
			}
		case tcell.KeyEnter:
			switch menuIndex {
			case 0:
				mapIndex = 0
				state = StateMapSelect
			case 1:
				mapIndex = 0
				state = StateMapSelect
			case 2:
				screen.Fini()
				os.Exit(0)
			}
		case tcell.KeyRune:
			switch ev.Rune() {
			case '1':
				mapIndex = 0
				state = StateMapSelect
			case '2':
				mapIndex = 0
				state = StateMapSelect
			case '3', 'q', 'Q':
				screen.Fini()
				os.Exit(0)
			}
		}
	case StateMapSelect:
		switch ev.Key() {
		case tcell.KeyLeft:
			if mapIndex > 0 {
				mapIndex--
			}
		case tcell.KeyRight:
			if mapIndex < len(maps)-1 {
				mapIndex++
			}
		case tcell.KeyEnter:
			startGame(mapIndex)
		case tcell.KeyEscape:
			state = StateMenu
		case tcell.KeyRune:
			switch ev.Rune() {
			case '1':
				if len(maps) > 0 {
					startGame(0)
				}
			case '2':
				if len(maps) > 1 {
					startGame(1)
				}
			}
		}
	case StatePlaying:
		switch ev.Key() {
		case tcell.KeyUp:
			if cursorY > 0 {
				cursorY--
			}
		case tcell.KeyDown:
			if cursorY < currentMap.Height-1 {
				cursorY++
			}
		case tcell.KeyLeft:
			if cursorX > 0 {
				cursorX--
			}
		case tcell.KeyRight:
			if cursorX < currentMap.Width-1 {
				cursorX++
			}
		case tcell.KeyEnter:
			buildTower(cursorX, cursorY, selectedTowerType)
		case tcell.KeyEscape:
			state = StateMenu
		case tcell.KeyRune:
			switch ev.Rune() {
			case '1':
				selectedTowerType = TowerSniper
			case '2':
				selectedTowerType = TowerSplash
			case '3':
				selectedTowerType = TowerSlow
			case 'q', 'Q':
				if selectedTower != nil {
					upgradeTower(selectedTower)
				}
			case 't', 'T':
				if selectedTower != nil {
					sellTower(selectedTower)
				}
			case 'n', 'N':
				if !waveInProgress && wave < maxWave {
					waveTimer = 0
				}
			}
		}
	case StateGameOver:
		switch ev.Key() {
		case tcell.KeyRune:
			switch ev.Rune() {
			case '1':
				startGame(mapIndex)
			case '2':
				state = StateMapSelect
			case '3', 'm', 'M':
				state = StateMenu
			}
		case tcell.KeyEnter:
			startGame(mapIndex)
		case tcell.KeyEscape:
			state = StateMenu
		}
	}
}

func main() {
	var err error
	screen, err = tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()
	screen.EnableMouse()
	screen.SetStyle(tcell.StyleDefault)

	initMaps()
	loadHighScores()
	rand.Seed(time.Now().UnixNano())
	state = StateMenu

	lastTime = time.Now()
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()
	eventCh := make(chan tcell.Event, 16)

	go func() {
		for {
			ev := screen.PollEvent()
			eventCh <- ev
		}
	}()

	for {
		select {
		case ev := <-eventCh:
			switch ev := ev.(type) {
			case *tcell.EventResize:
				screen.Sync()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyCtrlC {
					return
				}
				handleKey(ev)
			}
		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(lastTime).Seconds()
			lastTime = now
			if dt > 0.1 {
				dt = 0.1
			}
			if state == StatePlaying {
				update(dt)
			}

			screen.Clear()
			switch state {
			case StateMenu:
				drawMenu()
			case StateMapSelect:
				drawMapSelect()
			case StatePlaying:
				drawGame()
			case StateGameOver:
				drawGameOver()
			}
			screen.Show()
		}
	}
}
