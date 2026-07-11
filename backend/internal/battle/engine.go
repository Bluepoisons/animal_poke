package battle

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// runtimeFighter is mutable combat state.
type runtimeFighter struct {
	Fighter
	statuses      []ActiveStatus
	cooldowns     map[string]int
	controlStreak int
	shieldHP      int
	strategy      Strategy
	alive         bool
}

type simState struct {
	rng            *seedRNG
	round          int
	fighters       map[string]*runtimeFighter
	order          []string // stable id list
	playerIDs      []string
	enemyIDs       []string
	events         []Event
	metrics        ResultMetrics
	zeroDmgStreak  int
	cmdIdx         int
	commands       []Command
	playerStrategy Strategy
	endedBy        string
}

// Simulate runs an authoritative battle for seed + teams + commands.
// Enemy actions without matching commands are auto-selected deterministically.
func Simulate(seed string, players, enemies []Fighter, commands []Command) (Result, error) {
	if strings.TrimSpace(seed) == "" {
		return Result{}, fmt.Errorf("seed required")
	}
	if len(players) == 0 || len(enemies) == 0 {
		return Result{}, fmt.Errorf("both sides require at least one fighter")
	}
	if len(players) > MaxTeamSize || len(enemies) > MaxTeamSize {
		return Result{}, fmt.Errorf("team size exceeds %d", MaxTeamSize)
	}
	if len(commands) > MaxCommandsPerBattle {
		return Result{}, fmt.Errorf("too many commands")
	}

	st := &simState{
		rng:            newSeedRNG(seed),
		fighters:       make(map[string]*runtimeFighter),
		commands:       append([]Command(nil), commands...),
		playerStrategy: StrategyBalanced,
		events:         make([]Event, 0, 64),
	}

	for i := range players {
		f := players[i]
		f.Side = "player"
		if err := validateFighter(f); err != nil {
			return Result{}, fmt.Errorf("player[%d]: %w", i, err)
		}
		rf := newRuntime(f)
		st.fighters[rf.ID] = rf
		st.playerIDs = append(st.playerIDs, rf.ID)
		st.order = append(st.order, rf.ID)
	}
	for i := range enemies {
		f := enemies[i]
		f.Side = "enemy"
		if err := validateFighter(f); err != nil {
			return Result{}, fmt.Errorf("enemy[%d]: %w", i, err)
		}
		rf := newRuntime(f)
		st.fighters[rf.ID] = rf
		st.enemyIDs = append(st.enemyIDs, rf.ID)
		st.order = append(st.order, rf.ID)
	}

	for st.round = 1; st.round <= MaxRounds; st.round++ {
		if winner := st.sideWinner(); winner != "" {
			st.endedBy = "ko"
			return st.finish(seed, winner), nil
		}
		st.tickStartOfRound()
		actors := st.turnOrder()
		for _, id := range actors {
			rf := st.fighters[id]
			if rf == nil || !rf.alive {
				continue
			}
			if winner := st.sideWinner(); winner != "" {
				st.endedBy = "ko"
				return st.finish(seed, winner), nil
			}
			st.act(rf)
		}
		st.tickEndOfRound()
		if winner := st.sideWinner(); winner != "" {
			st.endedBy = "ko"
			return st.finish(seed, winner), nil
		}
	}
	st.endedBy = "timeout"
	st.round = MaxRounds
	// timeout: more remaining HP ratio wins, else draw
	return st.finish(seed, st.timeoutWinner()), nil
}

func validateFighter(f Fighter) error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("id required")
	}
	if f.MaxHP <= 0 {
		f.MaxHP = f.HP
	}
	if f.HP <= 0 || f.MaxHP <= 0 {
		return fmt.Errorf("hp must be positive")
	}
	if f.ATK < 0 || f.DEF < 0 || f.SPD < 0 {
		return fmt.Errorf("stats must be non-negative")
	}
	if len(f.SkillIDs) == 0 {
		return fmt.Errorf("at least one skill required")
	}
	for _, sid := range f.SkillIDs {
		if _, ok := SkillByID(sid); !ok {
			return fmt.Errorf("unknown skill %q", sid)
		}
	}
	return nil
}

func newRuntime(f Fighter) *runtimeFighter {
	if f.MaxHP <= 0 {
		f.MaxHP = f.HP
	}
	if f.HP > f.MaxHP {
		f.HP = f.MaxHP
	}
	if f.CRIT <= 0 {
		f.CRIT = 8
	}
	if f.EVA <= 0 {
		f.EVA = 5
	}
	if f.SkillLevels == nil {
		f.SkillLevels = map[string]int{}
	}
	for _, sid := range f.SkillIDs {
		if f.SkillLevels[sid] <= 0 {
			f.SkillLevels[sid] = DefaultSkillLevel
		}
		if f.SkillLevels[sid] > MaxSkillLevel {
			f.SkillLevels[sid] = MaxSkillLevel
		}
	}
	if f.Slot == "" {
		if role, ok := RoleByID(f.Role); ok {
			f.Slot = role.PreferredSlot
		} else {
			f.Slot = SlotMid
		}
	}
	return &runtimeFighter{
		Fighter:   f,
		statuses:  nil,
		cooldowns: map[string]int{},
		strategy:  StrategyBalanced,
		alive:     f.HP > 0,
	}
}

func (st *simState) finish(seed, winner string) Result {
	res := Result{
		WinnerSide:  winner,
		Rounds:      st.round,
		Events:      st.events,
		PlayerAlive: st.countAlive("player"),
		EnemyAlive:  st.countAlive("enemy"),
		RuleVersion: RuleVersion,
		Seed:        seed,
		CommandHash: HashCommands(st.commands),
		EndedBy:     st.endedBy,
		Metrics:     st.metrics,
	}
	if winner == "enemy" || winner == "draw" {
		res.FailureFactors = ExplainFailure(st)
	}
	return res
}

func (st *simState) countAlive(side string) int {
	n := 0
	for _, rf := range st.fighters {
		if rf.Side == side && rf.alive {
			n++
		}
	}
	return n
}

func (st *simState) sideWinner() string {
	p := st.countAlive("player")
	e := st.countAlive("enemy")
	if p == 0 && e == 0 {
		return "draw"
	}
	if p == 0 {
		return "enemy"
	}
	if e == 0 {
		return "player"
	}
	return ""
}

func (st *simState) timeoutWinner() string {
	var pHP, pMax, eHP, eMax int
	for _, rf := range st.fighters {
		if !rf.alive {
			continue
		}
		if rf.Side == "player" {
			pHP += rf.HP
			pMax += rf.MaxHP
		} else {
			eHP += rf.HP
			eMax += rf.MaxHP
		}
	}
	if pMax == 0 && eMax == 0 {
		return "draw"
	}
	pr := float64(pHP) / math.Max(1, float64(pMax))
	er := float64(eHP) / math.Max(1, float64(eMax))
	if pr > er+0.02 {
		return "player"
	}
	if er > pr+0.02 {
		return "enemy"
	}
	return "draw"
}

func (st *simState) turnOrder() []string {
	type pair struct {
		id  string
		spd int
	}
	var list []pair
	for _, id := range st.order {
		rf := st.fighters[id]
		if rf == nil || !rf.alive {
			continue
		}
		spd := st.effectiveSPD(rf)
		list = append(list, pair{id: id, spd: spd})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].spd == list[j].spd {
			return list[i].id < list[j].id
		}
		return list[i].spd > list[j].spd
	})
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.id
	}
	return out
}

func (st *simState) effectiveSPD(rf *runtimeFighter) int {
	mod := 1.0
	for _, s := range rf.statuses {
		if def, ok := StatusByID(s.ID); ok {
			mod += def.SPDMod * float64(s.Stacks)
		}
	}
	v := int(math.Round(float64(rf.SPD) * mod))
	if v < 1 {
		v = 1
	}
	return v
}

func (st *simState) effectiveATK(rf *runtimeFighter) int {
	mod := 1.0
	for _, s := range rf.statuses {
		if def, ok := StatusByID(s.ID); ok {
			mod += def.ATKMod * float64(s.Stacks)
		}
	}
	switch rf.strategy {
	case StrategyAggressive:
		mod += 0.10
	case StrategyDefensive:
		mod -= 0.10
	}
	mod *= positionOutMod(rf.Slot)
	v := int(math.Round(float64(rf.ATK) * mod))
	if v < 1 {
		v = 1
	}
	return v
}

func (st *simState) effectiveDEF(rf *runtimeFighter) int {
	mod := 1.0
	for _, s := range rf.statuses {
		if def, ok := StatusByID(s.ID); ok {
			mod += def.DEFMod * float64(s.Stacks)
		}
	}
	switch rf.strategy {
	case StrategyAggressive:
		mod -= 0.10
	case StrategyDefensive:
		mod += 0.10
	}
	v := int(math.Round(float64(rf.DEF) * mod))
	if v < 0 {
		v = 0
	}
	return v
}

func positionOutMod(slot SlotID) float64 {
	switch slot {
	case SlotFront:
		return PositionFrontDamageOut
	case SlotBack:
		return PositionBackDamageOut
	default:
		return PositionMidDamageOut
	}
}

func positionInMod(slot SlotID) float64 {
	switch slot {
	case SlotFront:
		return PositionFrontDamageIn
	case SlotBack:
		return PositionBackDamageIn
	default:
		return PositionMidDamageIn
	}
}

func (st *simState) tickStartOfRound() {
	for _, rf := range st.fighters {
		if !rf.alive {
			continue
		}
		for sid, cd := range rf.cooldowns {
			if cd > 0 {
				rf.cooldowns[sid] = cd - 1
			}
		}
	}
}

func (st *simState) tickEndOfRound() {
	for _, id := range st.order {
		rf := st.fighters[id]
		if rf == nil || !rf.alive {
			continue
		}
		// status ticks
		next := rf.statuses[:0]
		for _, s := range rf.statuses {
			if def, ok := StatusByID(s.ID); ok && def.TickDamagePct != 0 {
				amt := int(math.Round(float64(rf.MaxHP) * def.TickDamagePct * float64(s.Stacks)))
				if amt > 0 {
					st.applyRawDamage(rf, amt, "tick:"+string(s.ID))
					st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "tick", Text: fmt.Sprintf("%s 受到 %s %d", rf.Name, s.ID, amt), Damage: amt, Status: string(s.ID)})
				} else if amt < 0 {
					heal := -amt
					st.applyHeal(rf, heal)
					st.metrics.HealingDone += heal
					st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "regen", Text: fmt.Sprintf("%s 回复 %d", rf.Name, heal), Heal: heal, Status: string(s.ID)})
				}
			}
			s.Duration--
			if s.Duration > 0 && rf.alive {
				next = append(next, s)
			}
		}
		rf.statuses = next
		if rf.shieldHP > 0 {
			// shield duration handled via StatusShield presence; clear orphan shield
			if !hasStatus(rf, StatusShield) {
				rf.shieldHP = 0
			}
		}
	}
}

func hasStatus(rf *runtimeFighter, id StatusID) bool {
	for _, s := range rf.statuses {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (st *simState) isControlled(rf *runtimeFighter) (hard bool, rooted bool) {
	if hasStatus(rf, StatusImmune) {
		return false, false
	}
	for _, s := range rf.statuses {
		switch s.ID {
		case StatusStun:
			hard = true
		case StatusRoot:
			rooted = true
		}
	}
	return hard, rooted
}

func (st *simState) act(rf *runtimeFighter) {
	hard, rooted := st.isControlled(rf)
	if hard {
		rf.controlStreak++
		if rf.Side == "player" {
			st.metrics.ControlTurnsPlayer++
		} else {
			st.metrics.ControlTurnsEnemy++
		}
		st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "stun_skip", Text: fmt.Sprintf("%s 被眩晕，跳过行动", rf.Name)})
		st.breakControlIfNeeded(rf)
		return
	}
	// not hard-controlled: reset streak gradually
	if rf.controlStreak > 0 && !rooted {
		rf.controlStreak = 0
	}

	cmd := st.nextCommandFor(rf)
	if rooted && (cmd.Kind == CmdSkill || cmd.Kind == CmdUltimate) {
		st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "root_block", Text: fmt.Sprintf("%s 被禁锢，技能被打断", rf.Name)})
		cmd.Kind = CmdBasic
		cmd.SkillID = ""
	}

	switch cmd.Kind {
	case CmdStrategy:
		if cmd.Strategy != "" {
			rf.strategy = cmd.Strategy
			if rf.Side == "player" {
				st.playerStrategy = cmd.Strategy
			}
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "strategy", Text: fmt.Sprintf("%s 切换策略为 %s", rf.Name, cmd.Strategy)})
		}
		// strategy is free annotation; still perform a basic if no other action
		st.doBasic(rf, cmd.TargetID)
	case CmdSkill:
		if !st.doSkill(rf, cmd.SkillID, cmd.TargetID, false) {
			st.doBasic(rf, cmd.TargetID)
		}
	case CmdUltimate:
		if !st.doSkill(rf, "energy_burst", cmd.TargetID, true) {
			st.doBasic(rf, cmd.TargetID)
		}
	case CmdPass:
		st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "pass", Text: fmt.Sprintf("%s 待机", rf.Name)})
	default:
		st.doBasic(rf, cmd.TargetID)
	}
}

func (st *simState) breakControlIfNeeded(rf *runtimeFighter) {
	if rf.controlStreak < MaxControlStreak {
		return
	}
	// strip control statuses and grant immunity
	filtered := rf.statuses[:0]
	for _, s := range rf.statuses {
		if def, ok := StatusByID(s.ID); ok && def.IsControl {
			continue
		}
		if s.ID == StatusStun || s.ID == StatusRoot {
			continue
		}
		filtered = append(filtered, s)
	}
	rf.statuses = filtered
	rf.controlStreak = 0
	st.applyStatus(rf, StatusImmune, 1, ControlImmunityTurns, "softlock_break")
	st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "control_break", Text: fmt.Sprintf("%s 挣脱控制并获得短暂免疫", rf.Name), Status: string(StatusImmune)})
}

func (st *simState) nextCommandFor(rf *runtimeFighter) Command {
	// consume matching player commands in order; enemy always AI unless log has actor
	for st.cmdIdx < len(st.commands) {
		c := st.commands[st.cmdIdx]
		st.cmdIdx++
		if c.ActorID == "" || c.ActorID == rf.ID {
			if c.Kind == "" {
				c.Kind = CmdBasic
			}
			return c
		}
		// command for someone else — keep it only if that actor still upcoming; else drop as tamper noise
		// push back by not supporting multi-lookahead: re-queue via simple skip if actor dead
		if other := st.fighters[c.ActorID]; other != nil && other.alive && other.Side == rf.Side {
			// same side different unit: stash by inserting AI for current and rewinding is complex;
			// treat ordered log as global action stream: if not for this actor, generate AI and leave index rewound
			st.cmdIdx--
			break
		}
		// orphan command (dead/unknown) → skip (tamper resistant)
	}
	return st.aiCommand(rf)
}

func (st *simState) aiCommand(rf *runtimeFighter) Command {
	target := st.pickTarget(rf)
	// try ready skill
	var ready []string
	for _, sid := range rf.SkillIDs {
		if rf.cooldowns[sid] > 0 {
			continue
		}
		def, ok := SkillByID(sid)
		if !ok {
			continue
		}
		if def.Kind == SkillKindEnergy && rf.Energy < def.EnergyCost {
			continue
		}
		if def.Kind == SkillKindPassive {
			continue
		}
		ready = append(ready, sid)
	}
	if len(ready) > 0 {
		// deterministic pick weighted by base power
		idx := st.rng.IntN(len(ready))
		sid := ready[idx]
		def, _ := SkillByID(sid)
		if def.Kind == SkillKindEnergy {
			return Command{ActorID: rf.ID, Kind: CmdUltimate, SkillID: sid, TargetID: target}
		}
		return Command{ActorID: rf.ID, Kind: CmdSkill, SkillID: sid, TargetID: target}
	}
	return Command{ActorID: rf.ID, Kind: CmdBasic, TargetID: target}
}

func (st *simState) pickTarget(rf *runtimeFighter) string {
	oppSide := "enemy"
	if rf.Side == "enemy" {
		oppSide = "player"
	}
	var candidates []*runtimeFighter
	for _, id := range st.order {
		o := st.fighters[id]
		if o != nil && o.alive && o.Side == oppSide {
			candidates = append(candidates, o)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	// prefer front, then lowest HP
	sort.SliceStable(candidates, func(i, j int) bool {
		pi, pj := slotPriority(candidates[i].Slot), slotPriority(candidates[j].Slot)
		if pi != pj {
			return pi < pj
		}
		if candidates[i].HP != candidates[j].HP {
			return candidates[i].HP < candidates[j].HP
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0].ID
}

func slotPriority(s SlotID) int {
	switch s {
	case SlotFront:
		return 0
	case SlotMid:
		return 1
	default:
		return 2
	}
}

func (st *simState) doBasic(rf *runtimeFighter, targetID string) {
	target := st.resolveTarget(rf, targetID)
	if target == nil {
		return
	}
	dmg, crit, miss := st.computeDamage(rf, target, 1.0, rf.Element, false)
	if miss {
		st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "miss", Text: fmt.Sprintf("%s 的普攻被闪避", rf.Name)})
		st.noteZeroDamage(true)
		rf.Energy = min(MaxEnergy, rf.Energy+EnergyPerBasic/2)
		return
	}
	st.applyDamage(rf, target, dmg, "basic")
	txt := fmt.Sprintf("%s 普攻 %s 造成 %d", rf.Name, target.Name, dmg)
	if crit {
		txt += "（暴击）"
	}
	st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "basic", Text: txt, Damage: dmg})
	rf.Energy = min(MaxEnergy, rf.Energy+EnergyPerBasic)
	target.Energy = min(MaxEnergy, target.Energy+EnergyPerHit)
}

func (st *simState) doSkill(rf *runtimeFighter, skillID, targetID string, asUltimate bool) bool {
	if skillID == "" && asUltimate {
		skillID = "energy_burst"
	}
	def, ok := SkillByID(skillID)
	if !ok {
		return false
	}
	// ownership: skill must be equipped unless pure ultimate energy_burst always allowed if in list
	owned := false
	for _, sid := range rf.SkillIDs {
		if sid == skillID {
			owned = true
			break
		}
	}
	if !owned {
		return false
	}
	if rf.cooldowns[skillID] > 0 {
		return false
	}
	cost := def.EnergyCost
	if def.Kind == SkillKindEnergy || asUltimate {
		cost = EnergySkillCost
	}
	if cost > 0 && rf.Energy < cost {
		return false
	}

	level := rf.SkillLevels[skillID]
	if level <= 0 {
		level = 1
	}
	powerMul := 1.0 + 0.12*float64(level-1)
	cdCut := 0
	if level >= 3 {
		cdCut = 1
	}

	target := st.resolveTarget(rf, targetID)
	if target == nil && needsEnemyTarget(def) {
		return false
	}

	if cost > 0 {
		rf.Energy -= cost
	}

	applied := false
	for _, eff := range def.Effects {
		switch eff.Kind {
		case EffectDamage:
			if target == nil {
				continue
			}
			pow := eff.Power
			if pow <= 0 {
				pow = def.BasePower
			}
			pow *= powerMul
			element := def.Element
			if element == "" {
				element = rf.Element
			}
			ignoreEva := def.Kind == SkillKindEnergy || asUltimate
			dmg, crit, miss := st.computeDamage(rf, target, pow, element, ignoreEva)
			if miss {
				st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "miss", Text: fmt.Sprintf("%s 的 %s 被闪避", rf.Name, def.NameZH)})
				st.noteZeroDamage(true)
				continue
			}
			st.applyDamage(rf, target, dmg, "skill:"+skillID)
			txt := fmt.Sprintf("%s 使用 %s 对 %s 造成 %d", rf.Name, def.NameZH, target.Name, dmg)
			if crit {
				txt += "（暴击）"
			}
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "skill", Text: txt, Damage: dmg})
			applied = true
		case EffectHeal:
			dst := rf
			if !eff.TargetSelf && target != nil {
				dst = target
			}
			amt := int(math.Round(float64(dst.MaxHP) * eff.Power * powerMul))
			if amt < 1 {
				amt = 1
			}
			st.applyHeal(dst, amt)
			st.metrics.HealingDone += amt
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "heal", Text: fmt.Sprintf("%s 治疗 %s %d", rf.Name, dst.Name, amt), Heal: amt})
			applied = true
		case EffectStatus:
			dst := target
			if eff.TargetSelf {
				dst = rf
			}
			if dst == nil {
				continue
			}
			dur := eff.Duration
			if dur <= 0 {
				dur = 1
			}
			st.applyStatus(dst, eff.Status, 1, dur, skillID)
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "status", Text: fmt.Sprintf("%s 对 %s 施加 %s", rf.Name, dst.Name, eff.Status), Status: string(eff.Status)})
			applied = true
		case EffectShield:
			dst := rf
			if !eff.TargetSelf && target != nil {
				dst = target
			}
			sh := int(math.Round(float64(dst.MaxHP) * eff.Power * powerMul))
			if sh < 1 {
				sh = 1
			}
			dst.shieldHP += sh
			st.applyStatus(dst, StatusShield, 1, 2, skillID)
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "shield", Text: fmt.Sprintf("%s 获得护盾 %d", dst.Name, sh)})
			applied = true
		case EffectCleanse:
			dst := rf
			if !eff.TargetSelf && target != nil {
				dst = target
			}
			st.cleanseDebuffs(dst)
			st.events = append(st.events, Event{Round: st.round, ActorID: rf.ID, Kind: "cleanse", Text: fmt.Sprintf("%s 净化了负面状态", dst.Name)})
			applied = true
		}
	}

	cd := def.Cooldown - cdCut
	if cd < 0 {
		cd = 0
	}
	if cd > 0 {
		rf.cooldowns[skillID] = cd
	}
	if def.Kind != SkillKindEnergy && !asUltimate {
		rf.Energy = min(MaxEnergy, rf.Energy+EnergyPerBasic/2)
	}
	return applied || len(def.Effects) == 0
}

func needsEnemyTarget(def SkillDef) bool {
	for _, e := range def.Effects {
		if e.Kind == EffectDamage || (e.Kind == EffectStatus && !e.TargetSelf) {
			return true
		}
	}
	return false
}

func (st *simState) resolveTarget(rf *runtimeFighter, targetID string) *runtimeFighter {
	if targetID != "" {
		if t := st.fighters[targetID]; t != nil && t.alive && t.Side != rf.Side {
			return t
		}
	}
	id := st.pickTarget(rf)
	if id == "" {
		return nil
	}
	return st.fighters[id]
}

func (st *simState) computeDamage(atk, def *runtimeFighter, power float64, element ElementID, ignoreEva bool) (dmg int, crit bool, miss bool) {
	if !ignoreEva {
		eva := def.EVA
		if eva > 40 {
			eva = 40
		}
		if st.rng.IntN(100) < eva {
			return 0, false, true
		}
	}
	atkStat := st.effectiveATK(atk)
	defStat := st.effectiveDEF(def)
	base := float64(atkStat)*power - float64(defStat)*0.5
	floor := float64(atkStat) * MinDamageFloorRatio * power
	if base < floor {
		base = floor
	}
	// element
	em := elementMultiplier(element, def.Element)
	if em < 1.0 {
		st.metrics.ElementDisadvHits++
	}
	base *= em
	base *= positionInMod(def.Slot)

	critChance := atk.CRIT
	if critChance > 50 {
		critChance = 50
	}
	if st.rng.IntN(100) < critChance {
		crit = true
		base *= 1.75
	}
	dmg = int(math.Round(base))
	if dmg < 1 {
		dmg = 1
	}
	return dmg, crit, false
}

func elementMultiplier(atk, def ElementID) float64 {
	if atk == "" || def == "" {
		return 1.0
	}
	// fire > grass > water > fire; light <-> dark
	switch {
	case atk == ElementFire && def == ElementGrass,
		atk == ElementGrass && def == ElementWater,
		atk == ElementWater && def == ElementFire:
		return 1.5
	case atk == ElementFire && def == ElementWater,
		atk == ElementWater && def == ElementGrass,
		atk == ElementGrass && def == ElementFire:
		return 0.67
	case atk == ElementLight && def == ElementDark,
		atk == ElementDark && def == ElementLight:
		return 1.5
	default:
		return 1.0
	}
}

func (st *simState) applyDamage(src, dst *runtimeFighter, dmg int, reason string) {
	if dmg <= 0 {
		st.noteZeroDamage(true)
		return
	}
	st.noteZeroDamage(false)
	if dst.shieldHP > 0 {
		if dmg <= dst.shieldHP {
			dst.shieldHP -= dmg
			dmg = 0
		} else {
			dmg -= dst.shieldHP
			dst.shieldHP = 0
		}
	}
	if dmg > 0 {
		dst.HP -= dmg
		if src.Side == "player" {
			st.metrics.PlayerDamageDealt += dmg
		} else {
			st.metrics.EnemyDamageDealt += dmg
		}
	}
	if dst.HP <= 0 {
		dst.HP = 0
		dst.alive = false
		st.events = append(st.events, Event{Round: st.round, ActorID: src.ID, Kind: "ko", Text: fmt.Sprintf("%s 击败了 %s (%s)", src.Name, dst.Name, reason)})
	}
}

func (st *simState) applyRawDamage(dst *runtimeFighter, dmg int, reason string) {
	if dmg <= 0 || !dst.alive {
		return
	}
	dst.HP -= dmg
	if dst.Side == "player" {
		// ticks from either side count as env; attribute to enemy pressure for metrics lightly
		st.metrics.EnemyDamageDealt += dmg
	} else {
		st.metrics.PlayerDamageDealt += dmg
	}
	if dst.HP <= 0 {
		dst.HP = 0
		dst.alive = false
		st.events = append(st.events, Event{Round: st.round, ActorID: dst.ID, Kind: "ko", Text: fmt.Sprintf("%s 因 %s 倒下", dst.Name, reason)})
	}
}

func (st *simState) applyHeal(dst *runtimeFighter, amt int) {
	if !dst.alive || amt <= 0 {
		return
	}
	dst.HP += amt
	if dst.HP > dst.MaxHP {
		dst.HP = dst.MaxHP
	}
}

func (st *simState) applyStatus(dst *runtimeFighter, id StatusID, stacks, duration int, source string) {
	if !dst.alive {
		return
	}
	def, ok := StatusByID(id)
	if !ok {
		return
	}
	if def.IsControl && hasStatus(dst, StatusImmune) {
		st.events = append(st.events, Event{Round: st.round, ActorID: dst.ID, Kind: "immune", Text: fmt.Sprintf("%s 免疫了控制", dst.Name), Status: string(id)})
		return
	}
	if duration > def.MaxDuration && def.MaxDuration > 0 {
		duration = def.MaxDuration
	}
	maxStacks := def.MaxStacks
	if maxStacks <= 0 {
		maxStacks = MaxStatusStacks
	}
	for i := range dst.statuses {
		if dst.statuses[i].ID == id {
			dst.statuses[i].Stacks += stacks
			if dst.statuses[i].Stacks > maxStacks {
				dst.statuses[i].Stacks = maxStacks
			}
			if duration > dst.statuses[i].Duration {
				dst.statuses[i].Duration = duration
			}
			return
		}
	}
	if stacks > maxStacks {
		stacks = maxStacks
	}
	dst.statuses = append(dst.statuses, ActiveStatus{ID: id, Stacks: stacks, Duration: duration, Source: source})
}

func (st *simState) cleanseDebuffs(dst *runtimeFighter) {
	next := dst.statuses[:0]
	for _, s := range dst.statuses {
		switch s.ID {
		case StatusBurn, StatusBleed, StatusPoison, StatusStun, StatusRoot, StatusSlow:
			continue
		default:
			next = append(next, s)
		}
	}
	dst.statuses = next
}

func (st *simState) noteZeroDamage(zero bool) {
	if zero {
		st.zeroDmgStreak++
		if st.zeroDmgStreak >= ZeroDamageBreakAfter {
			// true damage break — apply to lowest HP living fighter
			var victim *runtimeFighter
			for _, id := range st.order {
				rf := st.fighters[id]
				if rf != nil && rf.alive {
					if victim == nil || rf.HP < victim.HP {
						victim = rf
					}
				}
			}
			if victim != nil {
				st.applyRawDamage(victim, TrueDamageBreak, "zero_damage_break")
				st.metrics.ZeroDamageBreaks++
				st.events = append(st.events, Event{Round: st.round, ActorID: victim.ID, Kind: "softlock_break", Text: fmt.Sprintf("零伤害破局：%s 受到 %d 真实伤害", victim.Name, TrueDamageBreak), Damage: TrueDamageBreak})
				st.endedBy = "softlock_break"
			}
			st.zeroDmgStreak = 0
		}
		return
	}
	st.zeroDmgStreak = 0
}

// BuildEnemyTeam materializes archetype members into fighters with stable ids.
func BuildEnemyTeam(archetypeID, seed string) ([]Fighter, error) {
	arch, ok := ArchetypeByID(archetypeID)
	if !ok {
		return nil, fmt.Errorf("unknown archetype %q", archetypeID)
	}
	// difficulty 1..5 → scale roughly 0.97..1.45
	diff := arch.Difficulty
	if diff < 1 {
		diff = 1
	}
	if diff > 5 {
		diff = 5
	}
	scale := 0.85 + 0.12*float64(diff)
	out := make([]Fighter, 0, len(arch.Members))
	for i, m := range arch.Members {
		id := fmt.Sprintf("enemy-%s-%d", archetypeID, i)
		r := newSeedRNG(seed + "|" + id)
		jitter := 0.95 + r.Float64()*0.10
		hp := int(math.Round(float64(m.HP) * jitter * scale))
		atk := int(math.Round(float64(m.ATK) * scale))
		def := int(math.Round(float64(m.DEF) * scale))
		spd := int(math.Round(float64(m.SPD) * (0.95 + 0.02*float64(diff))))
		out = append(out, Fighter{
			ID: id, Name: m.Name, Species: m.Species, Role: m.Role, Slot: m.Slot, Element: m.Element,
			MaxHP: hp, HP: hp, ATK: atk, DEF: def, SPD: spd, CRIT: 10 + diff, EVA: 6 + diff/2,
			SkillIDs: append([]string{}, m.SkillIDs...), Side: "enemy",
		})
	}
	return out, nil
}

// NormalizePlayerTeam applies role mods and validates skill loadout.
func NormalizePlayerTeam(in []Fighter) ([]Fighter, error) {
	if len(in) == 0 || len(in) > MaxTeamSize {
		return nil, fmt.Errorf("team size must be 1..%d", MaxTeamSize)
	}
	out := make([]Fighter, 0, len(in))
	seen := map[string]bool{}
	for i, f := range in {
		if f.ID == "" {
			f.ID = fmt.Sprintf("player-%d", i)
		}
		if seen[f.ID] {
			return nil, fmt.Errorf("duplicate fighter id %q", f.ID)
		}
		seen[f.ID] = true
		if role, ok := RoleByID(f.Role); ok {
			f.MaxHP = int(math.Round(float64(max(f.MaxHP, f.HP)) * role.HPMod))
			f.HP = f.MaxHP
			f.ATK = int(math.Round(float64(f.ATK) * role.ATKMod))
			f.DEF = int(math.Round(float64(f.DEF) * role.DEFMod))
			f.SPD = int(math.Round(float64(f.SPD) * role.SPDMod))
			if f.Slot == "" {
				f.Slot = role.PreferredSlot
			}
		}
		if f.MaxHP <= 0 {
			f.MaxHP = f.HP
		}
		if len(f.SkillIDs) == 0 {
			f.SkillIDs = []string{"claw_strike", "energy_burst"}
		}
		if len(f.SkillIDs) > 4 {
			f.SkillIDs = f.SkillIDs[:4]
		}
		f.Side = "player"
		if err := validateFighter(f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// HashCommands produces a stable digest for tamper detection.
func HashCommands(cmds []Command) string {
	b, _ := json.Marshal(cmds)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

// seedRNG is a tiny deterministic RNG from seed string (xorshift-ish via sha blocks).
type seedRNG struct {
	buf []byte
	s0  uint64
	s1  uint64
}

func newSeedRNG(seed string) *seedRNG {
	sum := sha256.Sum256([]byte(seed))
	s0 := binary.LittleEndian.Uint64(sum[0:8])
	s1 := binary.LittleEndian.Uint64(sum[8:16])
	if s0 == 0 {
		s0 = 0x9e3779b97f4a7c15
	}
	if s1 == 0 {
		s1 = 0xbf58476d1ce4e5b9
	}
	return &seedRNG{buf: sum[:], s0: s0, s1: s1}
}

func (r *seedRNG) next() uint64 {
	// xorshift128+
	s1 := r.s0
	s0 := r.s1
	r.s0 = s0
	s1 ^= s1 << 23
	r.s1 = s1 ^ s0 ^ (s1 >> 18) ^ (s0 >> 5)
	return r.s1 + s0
}

func (r *seedRNG) IntN(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func (r *seedRNG) Float64() float64 {
	// 53 bits mantissa
	return float64(r.next()>>11) / (1 << 53)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
