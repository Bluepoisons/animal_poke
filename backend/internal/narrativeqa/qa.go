// Package narrativeqa validates authored narrative content before publication.
package narrativeqa

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"animalpoke/backend/internal/narrativecatalog"
)

const maxBubbleRunes = 120
const maxPaths = 10000

// Diagnostic identifies an authored content failure without exposing player data.
type Diagnostic struct {
	Code     string `json:"code"`
	NodeID   string `json:"node_id,omitempty"`
	ChoiceID string `json:"choice_id,omitempty"`
	Flag     string `json:"flag,omitempty"`
	Locale   string `json:"locale,omitempty"`
	Message  string `json:"message"`
}

// Report is a deterministic CI artifact containing every currently reachable route.
type Report struct {
	Nodes       int          `json:"nodes"`
	Choices     int          `json:"choices"`
	Paths       [][]string   `json:"paths"`
	Endings     []string     `json:"endings"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Valid reports have no blocking diagnostics.
func (r Report) Valid() bool { return len(r.Diagnostics) == 0 }

type edge struct {
	choice narrativecatalog.ChoiceDef
	to     string
}

type routeState struct {
	flags     map[string]string
	knowledge map[string]bool
	rewards   map[string]bool
}

// Analyze validates the supplied catalog instead of reading a database so CI
// tests exactly the authored source that a new deployment will seed.
func Analyze(nodes []narrativecatalog.NodeDef, choices []narrativecatalog.ChoiceDef, annotations map[string]narrativecatalog.QAAnnotation, entries []string) Report {
	report := Report{Nodes: len(nodes), Choices: len(choices), Paths: [][]string{}, Endings: []string{}, Diagnostics: []Diagnostic{}}
	diagnosticKeys := map[string]bool{}
	add := func(d Diagnostic) {
		key := strings.Join([]string{d.Code, d.NodeID, d.ChoiceID, d.Flag, d.Locale, d.Message}, "\x00")
		if !diagnosticKeys[key] {
			diagnosticKeys[key] = true
			report.Diagnostics = append(report.Diagnostics, d)
		}
	}

	byID := make(map[string]narrativecatalog.NodeDef, len(nodes))
	for _, node := range nodes {
		if node.NodeID == "" {
			add(Diagnostic{Code: "node_id_missing", Message: "node id is required"})
			continue
		}
		if _, exists := byID[node.NodeID]; exists {
			add(Diagnostic{Code: "node_duplicate", NodeID: node.NodeID, Message: "node id must be unique"})
			continue
		}
		byID[node.NodeID] = node
		annotation, exists := annotations[node.NodeID]
		if !exists {
			add(Diagnostic{Code: "qa_metadata_missing", NodeID: node.NodeID, Message: "node needs QA metadata"})
			continue
		}
		if annotation.Locale != "zh-CN" {
			add(Diagnostic{Code: "locale_missing_or_invalid", NodeID: node.NodeID, Locale: annotation.Locale, Message: "authored node must declare zh-CN locale"})
		}
		if strings.TrimSpace(annotation.Summary) == "" || utf8.RuneCountInString(annotation.Summary) > maxBubbleRunes {
			add(Diagnostic{Code: "summary_invalid", NodeID: node.NodeID, Message: "summary must be present and fit one bubble"})
		}
		if len(annotation.EthicsLabels) == 0 {
			add(Diagnostic{Code: "ethics_label_missing", NodeID: node.NodeID, Message: "node needs at least one ethics label"})
		}
		if strings.TrimSpace(annotation.AssetLicense) == "" {
			add(Diagnostic{Code: "asset_license_missing", NodeID: node.NodeID, Message: "node needs an asset license declaration"})
		}
		if annotation.VoiceAsset != "" && strings.TrimSpace(annotation.Subtitle) == "" {
			add(Diagnostic{Code: "subtitle_missing", NodeID: node.NodeID, Message: "voiced node must include a subtitle"})
		}
		if utf8.RuneCountInString(node.Body) > maxBubbleRunes {
			add(Diagnostic{Code: "bubble_too_long", NodeID: node.NodeID, Locale: annotation.Locale, Message: fmt.Sprintf("body exceeds %d runes", maxBubbleRunes)})
		}
		if strings.Contains(strings.ToLower(node.Body), "todo") {
			add(Diagnostic{Code: "untranslated_or_placeholder_text", NodeID: node.NodeID, Locale: annotation.Locale, Message: "body contains a placeholder marker"})
		}
	}
	for nodeID := range annotations {
		if _, exists := byID[nodeID]; !exists {
			add(Diagnostic{Code: "qa_metadata_orphan", NodeID: nodeID, Message: "QA metadata does not map to a node"})
		}
	}

	adjacency := make(map[string][]edge, len(nodes))
	choiceIDs := map[string]bool{}
	for _, choice := range choices {
		if choice.ChoiceID == "" {
			add(Diagnostic{Code: "choice_id_missing", Message: "choice id is required"})
			continue
		}
		if choiceIDs[choice.ChoiceID] {
			add(Diagnostic{Code: "choice_duplicate", ChoiceID: choice.ChoiceID, Message: "choice id must be unique"})
			continue
		}
		choiceIDs[choice.ChoiceID] = true
		from, fromExists := byID[choice.FromNodeID]
		to, toExists := byID[choice.ToNodeID]
		if !fromExists || !toExists {
			add(Diagnostic{Code: "choice_reference_missing", ChoiceID: choice.ChoiceID, Message: "choice must reference existing from and to nodes"})
			continue
		}
		if strings.TrimSpace(choice.Label) == "" {
			add(Diagnostic{Code: "choice_label_missing", ChoiceID: choice.ChoiceID, Message: "choice label is required"})
		}
		fromQA, fromQAExists := annotations[from.NodeID]
		toQA, toQAExists := annotations[to.NodeID]
		if fromQAExists && toQAExists && fromQA.Timeline > toQA.Timeline {
			add(Diagnostic{Code: "timeline_reversal", ChoiceID: choice.ChoiceID, NodeID: choice.ToNodeID, Message: "choice moves backward in authored timeline"})
		}
		for key := range choice.Effects {
			if !strings.HasPrefix(key, "flag:") && !strings.HasPrefix(key, "rel:") && !strings.HasPrefix(key, "clue:") && !strings.HasPrefix(key, "knowledge:") && !strings.HasPrefix(key, "reward:") {
				add(Diagnostic{Code: "effect_key_invalid", ChoiceID: choice.ChoiceID, Flag: key, Message: "effect key must use an approved namespace"})
			}
		}
		adjacency[choice.FromNodeID] = append(adjacency[choice.FromNodeID], edge{choice: choice, to: choice.ToNodeID})
	}
	for nodeID := range adjacency {
		sort.Slice(adjacency[nodeID], func(i, j int) bool {
			if adjacency[nodeID][i].choice.SortOrder == adjacency[nodeID][j].choice.SortOrder {
				return adjacency[nodeID][i].choice.ChoiceID < adjacency[nodeID][j].choice.ChoiceID
			}
			return adjacency[nodeID][i].choice.SortOrder < adjacency[nodeID][j].choice.SortOrder
		})
	}

	for nodeID, node := range byID {
		annotation := annotations[nodeID]
		outgoing := len(adjacency[nodeID])
		if outgoing == 0 && !annotation.Terminal {
			add(Diagnostic{Code: "dead_end", NodeID: nodeID, Message: "non-terminal node has no outgoing choice"})
		}
		if outgoing > 0 && annotation.Terminal {
			add(Diagnostic{Code: "terminal_has_outgoing_choice", NodeID: nodeID, Message: "terminal node must not have outgoing choices"})
		}
		if node.Kind == "ending" && !annotation.Terminal {
			add(Diagnostic{Code: "ending_not_terminal", NodeID: nodeID, Message: "ending node must be terminal"})
		}
	}

	visited := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(string)
	visit = func(nodeID string) {
		if visiting[nodeID] {
			add(Diagnostic{Code: "cycle", NodeID: nodeID, Message: "narrative graph contains a cycle"})
			return
		}
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true
		visiting[nodeID] = true
		for _, next := range adjacency[nodeID] {
			visit(next.to)
		}
		delete(visiting, nodeID)
	}
	for _, entry := range entries {
		if _, exists := byID[entry]; !exists {
			add(Diagnostic{Code: "entry_missing", NodeID: entry, Message: "entry node is not defined"})
			continue
		}
		visit(entry)
	}
	for nodeID := range byID {
		if !visited[nodeID] {
			add(Diagnostic{Code: "isolated_node", NodeID: nodeID, Message: "node is unreachable from an authored entry"})
		}
	}

	var walk func(nodeID string, state routeState, path []string)
	walk = func(nodeID string, state routeState, path []string) {
		for _, traversed := range path {
			if traversed == nodeID {
				return
			}
		}
		if len(report.Paths) >= maxPaths {
			add(Diagnostic{Code: "path_limit_exceeded", Message: "authored graph creates too many paths"})
			return
		}
		annotation := annotations[nodeID]
		for _, required := range annotation.KnowledgeRequires {
			if !state.knowledge[required] {
				add(Diagnostic{Code: "knowledge_before_discovery", NodeID: nodeID, Flag: required, Message: "node requires knowledge that this path has not established"})
			}
		}
		for _, provided := range annotation.KnowledgeProvides {
			state.knowledge[provided] = true
		}
		path = append(path, nodeID)
		outgoing := adjacency[nodeID]
		if len(outgoing) == 0 {
			if annotation.Terminal {
				report.Paths = append(report.Paths, append([]string(nil), path...))
				report.Endings = append(report.Endings, nodeID)
			}
			return
		}
		for _, next := range outgoing {
			nextState := copyState(state)
			for key, value := range next.choice.Effects {
				switch {
				case strings.HasPrefix(key, "flag:"):
					name := strings.TrimPrefix(key, "flag:")
					encoded := fmt.Sprint(value)
					if old, exists := nextState.flags[name]; exists && old != encoded {
						add(Diagnostic{Code: "flag_conflict", ChoiceID: next.choice.ChoiceID, Flag: name, Message: "path assigns conflicting flag values"})
					}
					nextState.flags[name] = encoded
				case strings.HasPrefix(key, "knowledge:"):
					nextState.knowledge[strings.TrimPrefix(key, "knowledge:")] = true
				case strings.HasPrefix(key, "reward:"):
					reward := fmt.Sprint(value)
					if nextState.rewards[reward] {
						add(Diagnostic{Code: "reward_duplicate", ChoiceID: next.choice.ChoiceID, Flag: reward, Message: "path grants a reward more than once"})
					}
					nextState.rewards[reward] = true
				}
			}
			walk(next.to, nextState, path)
		}
	}
	for _, entry := range entries {
		if _, exists := byID[entry]; exists {
			walk(entry, routeState{flags: map[string]string{}, knowledge: map[string]bool{}, rewards: map[string]bool{}}, nil)
		}
	}

	sort.Strings(report.Endings)
	deduplicatedEndings := make([]string, 0, len(report.Endings))
	for _, ending := range report.Endings {
		if len(deduplicatedEndings) == 0 || deduplicatedEndings[len(deduplicatedEndings)-1] != ending {
			deduplicatedEndings = append(deduplicatedEndings, ending)
		}
	}
	report.Endings = deduplicatedEndings
	sort.Slice(report.Paths, func(i, j int) bool { return strings.Join(report.Paths[i], "/") < strings.Join(report.Paths[j], "/") })
	sort.Slice(report.Diagnostics, func(i, j int) bool {
		left := strings.Join([]string{report.Diagnostics[i].Code, report.Diagnostics[i].NodeID, report.Diagnostics[i].ChoiceID, report.Diagnostics[i].Flag, report.Diagnostics[i].Message}, "\x00")
		right := strings.Join([]string{report.Diagnostics[j].Code, report.Diagnostics[j].NodeID, report.Diagnostics[j].ChoiceID, report.Diagnostics[j].Flag, report.Diagnostics[j].Message}, "\x00")
		return left < right
	})
	return report
}

func copyState(state routeState) routeState {
	out := routeState{flags: make(map[string]string, len(state.flags)), knowledge: make(map[string]bool, len(state.knowledge)), rewards: make(map[string]bool, len(state.rewards))}
	for key, value := range state.flags {
		out.flags[key] = value
	}
	for key, value := range state.knowledge {
		out.knowledge[key] = value
	}
	for key, value := range state.rewards {
		out.rewards[key] = value
	}
	return out
}

// AnalyzeSeed is the CI entry point for the production authored catalog.
func AnalyzeSeed() Report {
	return Analyze(narrativecatalog.SeedNodes(), narrativecatalog.SeedChoices(), narrativecatalog.QAAnnotations(), narrativecatalog.QAEntryNodes())
}
