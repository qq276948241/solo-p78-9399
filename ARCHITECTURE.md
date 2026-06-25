# 终端塔防游戏 - 架构说明（给自己人看的大白话版）

## 先说结论

项目就两个 `.go` 文件，一个管调度一个管效果。编译出来就是一个 `towerdefense.exe`，双击就能玩。

```
project78/
├── main.go          # 主程序：流程调度、UI渲染、键盘输入、游戏循环
├── effects.go       # 效果系统：伤害、暴击、燃烧、减速等所有数值计算
├── go.mod           # Go 模块依赖
└── highscores.json  # 运行后自动生成的最高分存档
```

---

## 两个文件分别干啥

### main.go — 导演兼场务，只管流程
- **结构体定义**：所有游戏对象（地图、敌人、塔、子弹）长啥样
- **游戏循环**：每 33 毫秒（约 30 FPS）跑一轮：读键盘 → 算逻辑 → 画画面
- **状态机**：现在是在主菜单 / 选地图 / 打游戏 / 结算界面
- **键盘输入**：方向键、1/2/3、Q、T、回车、ESC 这些按键能干啥
- **UI 渲染**：用 tcell 库在终端画格子、画塔、画怪、画血条、画状态栏
- **地图数据**：两张地图的路径和可建造区域硬编码在 `createMap1()` / `createMap2()`

### effects.go — 数值策划兼战斗法师，只管怎么造成伤害
- **Effect 系统**：燃烧、减速这些持续效果怎么挂、怎么算、怎么过期
- **暴击逻辑**：狙击塔 5% 起步 + 5% 每级，暴击伤害 × 2
- **燃烧逻辑**：溅射塔打中的怪每秒掉血，持续 3 秒，升级加 dps
- **减速逻辑**：减速塔打中还会连锁扩散到周围 1 格的怪
- **伤害计算**：先算暴击 → 再算护甲减伤 → 四舍五入 → 保底 1 点

**核心原则**：`main.go` 里不出现具体的伤害数值公式，`effects.go` 里不出现键盘和渲染。

---

## 枚举类型一览

都在 `main.go` 顶部，用 `iota` 自动从 0 开始编号。

### CellType — 地图格子类型
| 常量 | 值 | 含义 | 显示 |
|---|---|---|---|
| `CellEmpty` | 0 | 空（暂时没用） | - |
| `CellPath` | 1 | 怪物行走路径 | 深灰点 `.` |
| `CellBuild` | 2 | 可以建塔的格子 | 浅灰井号 `#` |
| `CellStart` | 3 | 怪物出生点 | 绿字 `S` |
| `CellEnd` | 4 | 基地，走到这扣血 | 红字 `E` |

### TowerType — 塔的种类
| 常量 | 值 | 符号 | 颜色 | 特性 |
|---|---|---|---|---|
| `TowerSniper` | 0 | `S` | 蓝 | 狙击塔：高伤单体，可暴击 |
| `TowerSplash` | 1 | `P` | 黄 | 溅射塔：范围伤害，挂燃烧 |
| `TowerSlow` | 2 | `D` | 绿 | 减速塔：减速 + 连锁扩散 |

### EnemyType — 怪的种类
| 常量 | 值 | 符号 | 颜色 | 特性 |
|---|---|---|---|---|
| `EnemyNormal` | 0 | `g` | 绿 | 普通怪：标准数据 |
| `EnemyFast` | 1 | `f` | 黄 | 快速怪：血少跑得快 |
| `EnemyArmor` | 2 | `r` | 红 | 重甲怪：血厚减伤 40% |

### EffectType — 持续效果类型（effects.go 里）
| 常量 | 值 | 含义 |
|---|---|---|
| `EffectBurn` | 0 | 燃烧：每秒扣血 |
| `EffectSlow` | 1 | 减速：降低移动速度 |

### GameState — 游戏状态机
| 常量 | 值 | 含义 |
|---|---|---|
| `StateMenu` | 0 | 主菜单 |
| `StateMapSelect` | 1 | 选择地图 |
| `StatePlaying` | 2 | 游戏进行中 |
| `StateGameOver` | 3 | 游戏结束 / 胜利结算 |

---

## 核心结构体存什么

### MapDef — 一张地图的完整数据
```go
type MapDef struct {
    Name   string       // 地图名字，显示用
    Width  int          // 格子宽（目前 30）
    Height int          // 格子高（目前 15）
    Grid   [][]CellType // 二维数组，每个格子是什么类型
    Path   []Point      // 路径坐标列表，怪按这个顺序走
}
```
路径是连续的格子坐标，怪从 `Path[0]` 出发，一步步走到 `Path[last]`（基地）。

### Enemy — 一个怪
```go
type Enemy struct {
    Type           EnemyType   // 哪种怪
    PathIndex      int         // 走到路径的第几个格子了
    Progress       float64     // 在当前格子里走了多少（0~1）
    HP             int         // 当前血量
    MaxHP          int         // 最大血量
    Speed          float64     // 当前速度（会被减速效果改）
    BaseSpeed      float64     // 原始速度（减速恢复用）
    Reward         int         // 打死给多少金币
    ArmorReduction float64     // 护甲减伤百分比（重甲 0.4）
    Alive          bool        // 活着吗
    Effects        []Effect    // 身上挂的持续效果列表
}
```
**怪的位置不是存 (x,y)，而是存走到第几个格子 + 格子内进度**，这样移动时不用做碰撞检测，直接按路径推进。

### Tower — 一座塔
```go
type Tower struct {
    Type         TowerType // 哪种塔
    Level        int       // 等级 1~3
    X, Y         int       // 在哪个格子
    Damage       int       // 每次攻击伤害
    Range        float64   // 射程（格子数）
    FireRate     float64   // 每秒开火几次
    FireCooldown float64   // 距离下次开火还有几秒
    TotalCost    int       // 总共花了多少金币（卖塔返还一半）
    CritCount    int       // 狙击塔专属：暴击了多少次
    SplashRadius float64   // 溅射塔专属：爆炸半径
    SlowAmount   float64   // 减速塔专属：减速百分比
    SlowDuration float64   // 减速塔专属：减速持续秒数
}
```
不同塔用的字段不一样，比如狙击塔用不上 `SplashRadius`，但放一起省事。

### Projectile — 一发子弹
```go
type Projectile struct {
    X, Y          float64 // 当前像素位置（其实是格子坐标的小数版）
    TargetX, Y    float64 // 目标位置
    Target        *Enemy  // 目标怪指针（可以是 nil）
    Damage        int     // 伤害
    Speed         float64 // 飞行速度
    SourceTower   *Tower  // 哪个塔射的（命中时用）
    IsSplash      bool    // 是不是溅射弹
    SplashRadius  float64 // 溅射半径
    IsSlow        bool    // 是不是减速弹
    SlowAmount    float64 // 减速数值
    SlowDuration  float64 // 减速持续
    Alive         bool    // 还在飞吗
}
```
子弹飞出去后，每帧向目标靠近，碰到了就调用 `ApplyEffect()` 结算伤害。

### Effect — 一个持续效果（effects.go 里）
```go
type Effect struct {
    Type        EffectType  // 燃烧还是减速
    Remaining   float64     // 还剩几秒
    Value       float64     // 数值：燃烧=dps，减速=减速百分比
    SourceLevel int         // 挂上时塔的等级 — 数值已算好存 Value，每帧不再重算！
}
```
**重要！** `Value` 是 debuff 挂上那一刻就算好存进去的，比如塔 Lv1 挂了 2dps 的燃烧，哪怕塔升到 Lv2，这个已经挂着的燃烧还是 2dps。只有塔再次命中这个怪刷新 debuff 时，才会更新成新等级的数值。

---

## 一帧的更新循环（30 次/秒）

整个游戏在 `main()` 函数里跑一个大循环，每 33ms 来一遍：

```
  读键盘输入
     ↓
  handleKey() → 根据当前状态改变量（比如移动光标、建塔、切菜单）
     ↓
  如果是 Playing 状态 → 调用 update(dt)
     ↓
  update(dt) 三步走：
    1. 更新波次进度 → 该出怪就出怪
    2. 遍历所有怪 → UpdateEffects() 结算 debuff → moveEnemy() 往前走
    3. 遍历所有塔 → 冷却到了就找目标 → fireTower() 射子弹
    4. 遍历所有子弹 → 飞行 → 命中了就 resolveProjectileHit()
     ↓
  清屏 → 根据当前状态 drawXXX() 画界面 → 显示
```

### resolveProjectileHit() 命中后怎么走
```
  子弹命中
     ↓
  是溅射弹？→ ApplySplashAt(tower, x, y) → 范围伤害 + 挂燃烧
     ↓
  是单体弹？→ ApplyEffect(tower, target, damage)
                  ↓
            狙击塔先 RollCrit() 判定暴击
                  ↓
            DealDamage() 扣血（先暴击 ×2 → 再护甲减伤）
                  ↓
            溅射塔：给目标周围挂燃烧
            减速塔：给目标 + 周围 1 格挂减速
```

### 伤害计算顺序（重要，别搞反了）
1. 先算**暴击**：狙击塔有概率把基础伤害 × 2
2. 再算**护甲减伤**：伤害 × (1 - ArmorReduction)
3. **四舍五入** + 保底 1 点：避免小伤害被截断成 0

这个顺序写在 `DealDamage()` 和 `ApplyEffect()` 的注释里了，改代码时别瞎调。

---

## 状态机怎么切换

都在 `handleKey()` 里，根据当前 `state` 变量决定按键能干啥：

```
StateMenu (主菜单)
    ↓ 回车选"开始游戏"
StateMapSelect (选地图)
    ↓ 回车选地图
StatePlaying (打游戏)
    ↓ 血量归零 或 打完 20 波
StateGameOver (结算)
    ↓ 选"再来一局" → 回到 StatePlaying（同地图）
    ↓ 选"换地图" → 回到 StateMapSelect
    ↓ 选"回主菜单" → 回到 StateMenu
```

任何时候按 ESC 都能回上一级。

---

## 数据存在哪

### 地图数据
硬编码在 `main.go` 的 `createMap1()` 和 `createMap2()` 里。每张地图用两层循环画路径，把路径格子标成 `CellPath`，其他可建造格子标 `CellBuild`。

想加新地图？抄 `createMap2()` 改 `Path` 坐标就行，然后在 `initMaps()` 里 `append` 进去。

### 最高分数据
游戏结束时自动写入 `highscores.json`，存在可执行文件同级目录。格式：
```json
[
  {
    "map_name": "蜿蜒之路",
    "wave": 20,
    "kills": 150,
    "total_gold": 2500,
    "rating": "S",
    "date": "2026-06-26 15:30"
  }
]
```
最多存 10 条，按波次 > 击杀数排序。主菜单和结算界面会显示前 3 名。

---

## 想加新功能？大概改这些地方

### 加一种新塔（比如闪电塔，连锁攻击）
1. **`main.go` 加枚举**：`TowerType` 里加个 `TowerLightning`
2. **`getTowerStats()`**：加新塔的 Lv1/2/3 数值
3. **`getTowerSymbol/Color/Name()`**：加新塔的显示符号、颜色、名字
4. **`effects.go` 加逻辑**：写个 `ApplyLightningChain()` 函数处理连锁
5. **`ApplyEffect()`**：`switch tower.Type` 里加个 `case TowerLightning`
6. **`drawHUD()`**：效果统计行里加新塔的专属信息
7. **主菜单 tips**：加一行新塔说明（可选）

### 加一种新 debuff（比如中毒，减速 + 持续伤害）
1. **`effects.go` 加枚举**：`EffectType` 里加个 `EffectPoison`
2. **`Effect` 结构体**：如果需要新字段就加（大部分情况 Value + Remaining 够了）
3. **`NewPoisonEffect()`**：构造函数
4. **`UpdateEffects()`**：加个 `case EffectPoison` 处理每帧效果
5. **`HasEffect()` / `MaxSlowAmount()`**：如果渲染需要查询就加辅助函数
6. **`AddEffect()`**：同类刷新逻辑已经是通用的，不用改
7. **渲染**：`drawGame()` 里给中毒的怪加个视觉标记（比如底色紫）
8. **`CountActiveEffects()`**：已经通用统计，不用改

### 改数值平衡
- 塔的数值：`getTowerStats()` 里改数组
- 怪的数值：`createEnemy()` 里改
- 波次生成：`generateWaveEnemies()` 里改每波怪的数量
- 暴击率：`CritChance()` 里改公式
- 燃烧 dps：`SplashBurnDPS()` 里改公式

### 加新地图
- 抄 `createMap2()`，改 `Path` 数组的坐标就行
- `initMaps()` 里 `maps = make([]MapDef, 2)` 改成 3，`maps[2] = createMap3()`
- 主菜单和选地图界面会自动适配（不需要手动改 UI）

---

## 开发小 Tips

1. **先编译再改逻辑**：`go build .` 先确保没语法错误
2. **数值先写死再调平衡**：别一开始就纠结数值对不对，能跑起来最重要
3. **加新东西先加枚举**：所有类型系统都靠 `iota` 枚举驱动，加了枚举编译器会提醒你哪漏了
4. **`effects.go` 里别碰 UI**：如果发现要在 `effects.go` 里调用 `screen.SetContent`，那肯定设计错了
5. **`main.go` 里别写公式**：伤害计算、概率判定全放 `effects.go`，保持分层
