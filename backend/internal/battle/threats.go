package battle

import "fmt"

// AnalyzeThreats produces pre-battle readable threats for a player team vs archetype.
func AnalyzeThreats(players []Fighter, archetypeID string) []Threat {
	arch, ok := ArchetypeByID(archetypeID)
	if !ok {
		return []Threat{{
			Code: "unknown_archetype", Severity: "medium",
			TextZH: "未知敌方类型，请谨慎应战", TextEN: "Unknown enemy archetype",
		}}
	}
	var threats []Threat
	// archetype-level tags
	for _, tag := range arch.ThreatTags {
		switch tag {
		case "high_hp", "high_atk":
			threats = append(threats, Threat{
				Code: "physical_pressure", Severity: "high",
				TextZH: "敌方高攻高血，前排压力大", TextEN: "Enemy has high ATK/HP pressure on frontline",
				Counter: "带坦克+治疗，或用控制打断",
			})
		case "burst", "low_hp":
			threats = append(threats, Threat{
				Code: "burst_threat", Severity: "high",
				TextZH: "敌方爆发极高但身板脆", TextEN: "Enemy bursts hard but is fragile",
				Counter: "先手秒杀或护盾硬抗一波",
			})
		case "high_def", "stall":
			threats = append(threats, Threat{
				Code: "stall_wall", Severity: "medium",
				TextZH: "敌方高防磨战，普攻效率低", TextEN: "High DEF stall — basic attacks inefficient",
				Counter: "灼烧/中毒等百分比伤害，或攻击增益",
			})
		case "stun", "root", "slow":
			threats = append(threats, Threat{
				Code: "control_chain", Severity: "high",
				TextZH: "敌方擅长控制链，输出窗口受限", TextEN: "Enemy control chain limits your windows",
				Counter: "净化、高SPD、利用连控免疫窗口",
			})
		case "multi_hit", "attrition":
			threats = append(threats, Threat{
				Code: "swarm", Severity: "medium",
				TextZH: "多目标骚扰，分散火力", TextEN: "Multiple skirmishers split your focus",
				Counter: "优先击杀高速单位",
			})
		case "regen", "sustain":
			threats = append(threats, Threat{
				Code: "sustain_boss", Severity: "high",
				TextZH: "敌方持续回复，拖回合会吃亏", TextEN: "Enemy regenerates — long fights favor them",
				Counter: "爆发窗口集火治疗位",
			})
		}
	}

	// element disadvantages
	playerElems := map[ElementID]int{}
	for _, p := range players {
		playerElems[p.Element]++
	}
	for _, m := range arch.Members {
		for pe := range playerElems {
			mul := elementMultiplier(m.Element, pe)
			if mul > 1.0 {
				threats = append(threats, Threat{
					Code: fmt.Sprintf("elem_%s_vs_%s", m.Element, pe), Severity: "medium",
					TextZH:  fmt.Sprintf("敌方 %s 元素克制你的 %s", m.Element, pe),
					TextEN:  fmt.Sprintf("Enemy %s beats your %s", m.Element, pe),
					Counter: "换克制元素技能或提高防御",
				})
			}
		}
	}

	// role coverage
	hasTank, hasSupport, hasControl := false, false, false
	for _, p := range players {
		switch p.Role {
		case RoleTank:
			hasTank = true
		case RoleSupport:
			hasSupport = true
		case RoleControl:
			hasControl = true
		}
	}
	if !hasTank && arch.Difficulty >= 3 {
		threats = append(threats, Threat{
			Code: "no_tank", Severity: "medium",
			TextZH: "队伍缺少坦克，高难关承伤不足", TextEN: "No tank — fragile vs hard content",
			Counter: "至少一名前排坦克",
		})
	}
	if !hasSupport && (containsTag(arch.ThreatTags, "attrition") || containsTag(arch.ThreatTags, "sustain")) {
		threats = append(threats, Threat{
			Code: "no_support", Severity: "medium",
			TextZH: "缺少治疗/辅助，续航关卡吃亏", TextEN: "No support — weak into attrition",
			Counter: "携带 heal_lick / pack_regen",
		})
	}
	if !hasControl && containsTag(arch.ThreatTags, "burst") {
		threats = append(threats, Threat{
			Code: "no_control", Severity: "low",
			TextZH: "缺少控制，难以打断敌方爆发", TextEN: "No control to interrupt enemy burst",
			Counter: "mud_trap / dark_fang 等短控",
		})
	}

	// always include archetype counter hint
	threats = append(threats, Threat{
		Code: "archetype_hint", Severity: "low",
		TextZH: arch.CounterHint, TextEN: arch.CounterHint,
	})

	return dedupeThreats(threats)
}

func containsTag(tags []string, t string) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}

func dedupeThreats(in []Threat) []Threat {
	seen := map[string]bool{}
	out := make([]Threat, 0, len(in))
	for _, t := range in {
		if seen[t.Code] {
			continue
		}
		seen[t.Code] = true
		out = append(out, t)
	}
	return out
}

// ExplainFailure builds weighted post-battle factors from simulation metrics/events.
func ExplainFailure(st *simState) []FailureFactor {
	var factors []FailureFactor
	m := st.metrics
	if m.ControlTurnsPlayer >= 4 {
		factors = append(factors, FailureFactor{
			Code: "over_controlled", Weight: m.ControlTurnsPlayer,
			TextZH: fmt.Sprintf("被控制 %d 回合，输出窗口不足", m.ControlTurnsPlayer),
			TextEN: fmt.Sprintf("Controlled for %d turns — not enough damage windows", m.ControlTurnsPlayer),
		})
	}
	if m.ElementDisadvHits >= 3 {
		factors = append(factors, FailureFactor{
			Code: "element_disadvantage", Weight: m.ElementDisadvHits,
			TextZH: "元素克制劣势，有效伤害偏低",
			TextEN: "Element disadvantage reduced your damage",
		})
	}
	if m.EnemyDamageDealt > m.PlayerDamageDealt*3/2 && m.PlayerDamageDealt > 0 {
		factors = append(factors, FailureFactor{
			Code: "out_damaged", Weight: m.EnemyDamageDealt - m.PlayerDamageDealt,
			TextZH: "承伤远高于输出，阵容生存不足",
			TextEN: "Took far more damage than dealt — survivability too low",
		})
	}
	if m.HealingDone == 0 && st.round >= 8 {
		factors = append(factors, FailureFactor{
			Code: "no_healing", Weight: 5,
			TextZH: "整场无有效治疗，持久战难以为继",
			TextEN: "No healing landed — long fights collapsed",
		})
	}
	if st.endedBy == "timeout" {
		factors = append(factors, FailureFactor{
			Code: "timeout", Weight: 8,
			TextZH: "超时未击破敌方，输出效率不足",
			TextEN: "Timed out before finishing the enemy",
		})
	}
	// frontline died early?
	for _, id := range st.playerIDs {
		rf := st.fighters[id]
		if rf != nil && rf.Slot == SlotFront && !rf.alive && st.round <= 6 {
			factors = append(factors, FailureFactor{
				Code: "frontline_collapse", Weight: 7,
				TextZH: "前排过早倒下，后排被集火",
				TextEN: "Frontline collapsed early",
			})
			break
		}
	}
	if len(factors) == 0 {
		factors = append(factors, FailureFactor{
			Code: "generic_loss", Weight: 1,
			TextZH: "战力或策略不足，可尝试推荐队伍或调整技能",
			TextEN: "Power or strategy shortfall — try a recommended team",
		})
	}
	// sort by weight desc
	for i := 0; i < len(factors); i++ {
		for j := i + 1; j < len(factors); j++ {
			if factors[j].Weight > factors[i].Weight {
				factors[i], factors[j] = factors[j], factors[i]
			}
		}
	}
	if len(factors) > 5 {
		factors = factors[:5]
	}
	return factors
}
