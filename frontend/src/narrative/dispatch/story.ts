/** AP-104: dispatch timer → seeded branching observation stories (no pet harm). */

export type DestinationId = 'riverside' | 'park_edge' | 'market_roof' | 'library_yard'

export interface DispatchStoryChoice {
  id: string
  label: string
  /** Risk/reward tag for daily mission UX */
  risk: 'safe' | 'curious' | 'bold'
  outcomeId: string
}

export interface DispatchStoryNode {
  id: string
  text: string
  choices?: DispatchStoryChoice[]
}

export interface DispatchStoryOutcome {
  id: string
  epilogue: string
  /** Server still owns gold; this is flavor + research clue only */
  clueId?: string
  rewardHint: 'standard' | 'bonus_clue' | 'soft'
}

export interface DispatchStoryDef {
  destination: DestinationId
  title: string
  companionTrait: string
  nodes: DispatchStoryNode[]
  outcomes: DispatchStoryOutcome[]
  /** Main event chain id for 3-day dedupe */
  chainId: string
}

export interface DispatchRunInput {
  seed: string
  dayKey: string // YYYY-MM-DD
  destination: DestinationId
  /** Last 3 days chain ids for dedupe */
  recentChains: string[]
}

export interface DispatchRunResult {
  chainId: string
  destination: DestinationId
  title: string
  beats: string[]
  choiceId: string
  outcome: DispatchStoryOutcome
  /** Offline-complete friendly claim token (deterministic) */
  claimToken: string
}

const STORIES: DispatchStoryDef[] = [
  {
    destination: 'riverside',
    title: '河堤观察',
    companionTrait: '灵敏听觉',
    chainId: 'chain.river.tide',
    nodes: [
      {
        id: 'n1',
        text: '潮位牌旁落着新鲜爪印，像在绕开围挡。',
        choices: [
          { id: 'c_map', label: '画下爪印走向', risk: 'safe', outcomeId: 'o_map' },
          { id: 'c_listen', label: '停一停听水声', risk: 'curious', outcomeId: 'o_listen' },
        ],
      },
    ],
    outcomes: [
      { id: 'o_map', epilogue: '你记下绕行路线，没有惊扰任何生灵。', clueId: 'clue.bypass', rewardHint: 'bonus_clue' },
      { id: 'o_listen', epilogue: '水声里有闸门节奏，像城市的另一枚时钟。', clueId: 'clue.gate', rewardHint: 'standard' },
    ],
  },
  {
    destination: 'park_edge',
    title: '公园边角',
    companionTrait: '耐走',
    chainId: 'chain.park.shade',
    nodes: [
      {
        id: 'n1',
        text: '树荫下有人留下研究便签：「影子比昨天短」。',
        choices: [
          { id: 'c_measure', label: '对照自己的影子', risk: 'safe', outcomeId: 'o_measure' },
          { id: 'c_collect', label: '收集便签编号', risk: 'curious', outcomeId: 'o_collect' },
        ],
      },
    ],
    outcomes: [
      { id: 'o_measure', epilogue: '季节在脚边挪动，你补了一条日照记录。', clueId: 'clue.sun', rewardHint: 'standard' },
      { id: 'o_collect', epilogue: '编号拼出「午后课堂」——社区观察活动预告。', clueId: 'clue.class', rewardHint: 'bonus_clue' },
    ],
  },
  {
    destination: 'market_roof',
    title: '市集屋顶通道',
    companionTrait: '平衡感',
    chainId: 'chain.market.wind',
    nodes: [
      {
        id: 'n1',
        text: '风把一张活动海报吹到你脚边，油墨未干。',
        choices: [
          { id: 'c_read', label: '读海报（安全）', risk: 'safe', outcomeId: 'o_read' },
          { id: 'c_trace', label: '看油墨来向', risk: 'bold', outcomeId: 'o_trace' },
        ],
      },
    ],
    outcomes: [
      { id: 'o_read', epilogue: '海报写着知识挑战日，无需夜间外出。', rewardHint: 'soft' },
      { id: 'o_trace', epilogue: '油墨气味指向文印店——白天可回访。', clueId: 'clue.print', rewardHint: 'bonus_clue' },
    ],
  },
  {
    destination: 'library_yard',
    title: '图书馆小院',
    companionTrait: '安静',
    chainId: 'chain.lib.pages',
    nodes: [
      {
        id: 'n1',
        text: '窗台上夹着一张旧借阅条，背面写着「沿河」。',
        choices: [
          { id: 'c_copy', label: '抄下书名', risk: 'safe', outcomeId: 'o_copy' },
          { id: 'c_ask', label: '问管理员是否公开', risk: 'curious', outcomeId: 'o_ask' },
        ],
      },
    ],
    outcomes: [
      { id: 'o_copy', epilogue: '书名指向城市水道史，适合书桌线阅读。', clueId: 'clue.book', rewardHint: 'standard' },
      { id: 'o_ask', epilogue: '管理员笑着说：公开区已经放了复印件。', rewardHint: 'soft' },
    ],
  },
]

function hashSeed(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

function mulberry32(a: number) {
  return function () {
    let t = (a += 0x6d2b79f5)
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function listDestinations(): DestinationId[] {
  return STORIES.map((s) => s.destination)
}

/** Deterministic story run; avoids repeating recent main chains when alternatives exist. */
export function runDispatchStory(input: DispatchRunInput): DispatchRunResult {
  const rng = mulberry32(hashSeed(`${input.seed}|${input.dayKey}|${input.destination}`))
  let pool = STORIES.filter((s) => s.destination === input.destination)
  if (pool.length === 0) pool = STORIES
  // Prefer non-recent chain
  const fresh = pool.filter((s) => !input.recentChains.includes(s.chainId))
  const def = (fresh.length ? fresh : pool)[Math.floor(rng() * (fresh.length ? fresh.length : pool.length))]

  const node = def.nodes[0]
  const choices = node.choices ?? []
  const choice = choices[Math.floor(rng() * choices.length)] ?? choices[0]
  const outcome = def.outcomes.find((o) => o.id === choice.outcomeId) ?? def.outcomes[0]

  const claimToken = `dispatch:${input.dayKey}:${def.chainId}:${choice.id}:${hashSeed(input.seed).toString(16)}`

  return {
    chainId: def.chainId,
    destination: def.destination,
    title: def.title,
    beats: [node.text, `选择：${choice.label}`, outcome.epilogue],
    choiceId: choice.id,
    outcome,
    claimToken,
  }
}

/** Ensure three consecutive days can avoid same chain when multiple destinations used. */
export function planThreeDayChains(seed: string, dests: DestinationId[]): string[] {
  const recent: string[] = []
  const out: string[] = []
  for (let d = 0; d < 3; d++) {
    const dayKey = `2026-07-0${d + 1}`
    const r = runDispatchStory({
      seed,
      dayKey,
      destination: dests[d % dests.length],
      recentChains: recent.slice(-3),
    })
    out.push(r.chainId)
    recent.push(r.chainId)
  }
  return out
}
