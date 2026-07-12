/**
 * AP-116 · Season 1 architecture (data-driven).
 * Theme: 谁有权替一座城市讲故事
 *
 * Consumers resolve beats/routes from this pack — no page-level hard-coded branches.
 */

export type ChapterId =
  | 'prologue.blank_page'
  | 'ch01.alley_echo'
  | 'ch02.along_river_sleepless'
  | 'ch03.rain_on_eaves'
  | 'ch04.map_blank'
  | 'finale.who_tells_the_city'

export type EmotionTag =
  | 'curiosity'
  | 'tension'
  | 'empathy'
  | 'doubt'
  | 'absence'
  | 'agency'
  | 'catharsis'

export type RouteKind = 'main' | 'optional' | 'home_mode' | 'no_camera' | 'bad_weather'

export interface ChapterBeat {
  id: string
  /** 开端 | 升级 | 转折 | 余韵 */
  kind: 'open' | 'escalate' | 'turn' | 'afterglow'
  title: string
  summary: string
  emotion: EmotionTag
  /** Minutes soft budget */
  minutes: number
}

export interface ChapterCastArc {
  characterId: string
  /** Visible change this chapter */
  change: string
  /** Knowledge they must NOT yet have */
  knowledgeBlock?: string
}

export interface ChapterChoice {
  id: string
  prompt: string
  /** No single correct answer */
  options: { id: string; label: string; delayedEcho: string }[]
  /** Chapter that surfaces the echo */
  echoIn: ChapterId
}

export interface ChapterRoutes {
  main: string
  optional: string[]
  /** Completable without real animal sightings */
  noAnimal: string
  homeMode: string
  noCamera: string
  badWeather: string
}

export interface SeasonChapter {
  id: ChapterId
  order: number
  title: string
  /** Independent theme that still advances seasonal suspense */
  themeQuestion: string
  emotionCurve: EmotionTag[]
  budgetMinutes: { min: number; max: number }
  beats: ChapterBeat[]
  cast: ChapterCastArc[]
  /** City exploration node (coarse place label only) */
  exploreNode: { id: string; placeLabel: string; activity: string }
  /** Collection / observation trigger (never rare-animal gated) */
  collectionTrigger: { id: string; description: string; minObservations: number }
  characterConflict: string
  choice: ChapterChoice
  unlocksNext: ChapterId | null
  routes: ChapterRoutes
  /** Must be true for QA gate */
  passableWithoutRareAnimal: true
  passableHomeMode: true
}

export interface SeasonOutcome {
  id: string
  title: string
  /** Driven by relations + choices + how notes are handled — not collect rate */
  requires: {
    minTrustSum?: number
    choiceIds?: string[]
    noteHandling?: 'preserve_gaps' | 'overwrite' | 'multivocal'
  }
  summary: string
  independentOfCollectionRate: true
}

export interface Season1Architecture {
  id: 'season1.city_journal'
  title: string
  coreTheme: string
  version: string
  prologueBudgetMinutes: { min: number; max: number }
  chapterBudgetMinutes: { min: number; max: number }
  chapters: SeasonChapter[]
  /** Dependency edges chapterId -> prerequisite chapterIds */
  dependencyGraph: Record<ChapterId, ChapterId[]>
  outcomes: SeasonOutcome[]
  /** Content volume metrics for production planning */
  contentScale: {
    totalMainMinutes: { min: number; max: number }
    minChoices: number
    minDelayedEchoes: number
    minHomeModeRoutes: number
  }
}

export const season1Architecture: Season1Architecture = {
  id: 'season1.city_journal',
  title: '第一季 · 城市手账',
  coreTheme: '谁有权替一座城市讲故事',
  version: '1',
  prologueBudgetMinutes: { min: 15, max: 20 },
  chapterBudgetMinutes: { min: 30, max: 45 },
  chapters: [
    {
      id: 'prologue.blank_page',
      order: 0,
      title: '第一张空白页',
      themeQuestion: '当同一条观察记录出现互相矛盾的注释，谁在改写手账？',
      emotionCurve: ['curiosity', 'doubt', 'agency'],
      budgetMinutes: { min: 15, max: 20 },
      beats: [
        {
          id: 'p.open',
          kind: 'open',
          title: '空白页与训练观察',
          summary: '用训练素材完成首个观察，手账出现第二行陌生注释。',
          emotion: 'curiosity',
          minutes: 4,
        },
        {
          id: 'p.escalate',
          kind: 'escalate',
          title: '矛盾注释',
          summary: '档案员与街拍者对同一画面给出不同解读。',
          emotion: 'doubt',
          minutes: 5,
        },
        {
          id: 'p.turn',
          kind: 'turn',
          title: '没有标准答案的选择',
          summary: '玩家决定保留两行注释还是合并成单一叙述。',
          emotion: 'agency',
          minutes: 4,
        },
        {
          id: 'p.afterglow',
          kind: 'afterglow',
          title: '悬念纸条',
          summary: '手账助手留下「巷口有回声」的便签，开启第一章。',
          emotion: 'curiosity',
          minutes: 2,
        },
      ],
      cast: [
        {
          characterId: 'archivist',
          change: '首次出场：谨慎、偏好可溯源记录',
          knowledgeBlock: '不知终章展览存在',
        },
        {
          characterId: 'street_photographer',
          change: '首次出场：强调现场感与主观取景',
        },
        {
          characterId: 'journal_aide',
          change: '虚构助手：引导但不替玩家下结论',
        },
      ],
      exploreNode: {
        id: 'exp.training_desk',
        placeLabel: '室内训练台',
        activity: '完成一次无需外出的观察',
      },
      collectionTrigger: {
        id: 'col.first_note',
        description: '任意一次成功观察或训练观察写入手账',
        minObservations: 1,
      },
      characterConflict: '档案员要求删掉「不专业」注释；街拍者要求保留现场语气。',
      choice: {
        id: 'choice.prologue.keep_both',
        prompt: '面对矛盾注释，你怎么处理？',
        options: [
          {
            id: 'keep_both',
            label: '两行都留着',
            delayedEcho: '第一章巷口居民会引用「你愿意保留空白」。',
          },
          {
            id: 'merge_single',
            label: '合并成单一叙述',
            delayedEcho: '档案员在第三章更信任你，但街拍者疏远。',
          },
          {
            id: 'flag_unverified',
            label: '标记为未核验',
            delayedEcho: '规划研究者在第二章邀请你一起做核验清单。',
          },
        ],
        echoIn: 'ch01.alley_echo',
      },
      unlocksNext: 'ch01.alley_echo',
      routes: {
        main: '训练观察 → 矛盾注释 → 选择 → 便签悬念',
        optional: ['重读注释对照', '查看同意与权限说明'],
        noAnimal: '训练素材包（猫/狗/鹅静帧）',
        homeMode: '全程桌面训练，无定位无相机真机',
        noCamera: '图库训练帧 / 合成帧',
        badWeather: '不适用（室内）',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
    {
      id: 'ch01.alley_echo',
      order: 1,
      title: '巷口的回声',
      themeQuestion: '老街共享空间里，谁的记忆可以上墙？',
      emotionCurve: ['empathy', 'tension', 'agency'],
      budgetMinutes: { min: 30, max: 45 },
      beats: [
        {
          id: 'c1.open',
          kind: 'open',
          title: '巷口告示栏',
          summary: '居民、商户与摄影者对同一告示栏各写一段。',
          emotion: 'empathy',
          minutes: 8,
        },
        {
          id: 'c1.escalate',
          kind: 'escalate',
          title: '空间占用冲突',
          summary: '摊位扩展与通行、拍摄与隐私互相挤压。',
          emotion: 'tension',
          minutes: 10,
        },
        {
          id: 'c1.turn',
          kind: 'turn',
          title: '价值选择',
          summary: '支持「多声道告示」还是「统一管理告示」。',
          emotion: 'agency',
          minutes: 8,
        },
        {
          id: 'c1.afterglow',
          kind: 'afterglow',
          title: '回声预告',
          summary: '选择影响滨水章节谁先开口。',
          emotion: 'curiosity',
          minutes: 4,
        },
      ],
      cast: [
        { characterId: 'archivist', change: '开始收集居民口述，但仍排斥「情绪化」措辞' },
        { characterId: 'street_photographer', change: '承认取景可能冒犯店主' },
        { characterId: 'urban_planner', change: '首次出场：用可达性与安全数据说话' },
        { characterId: 'journal_aide', change: '提示 Home Mode 可用热线记录替代街访' },
      ],
      exploreNode: {
        id: 'exp.old_street',
        placeLabel: '老街巷口（粗粒度）',
        activity: '观察告示栏与通行流线（可桌面模拟）',
      },
      collectionTrigger: {
        id: 'col.alley_voices',
        description: '收集 ≥2 条不同角色对巷口的描述碎片',
        minObservations: 2,
      },
      characterConflict: '商户要清静、摄影者要画面、居民要通行。',
      choice: {
        id: 'choice.ch01.board_policy',
        prompt: '告示栏规则由谁定？',
        options: [
          {
            id: 'multivocal_board',
            label: '多声道并存',
            delayedEcho: '终章展览默认展示多声部墙。',
          },
          {
            id: 'managed_board',
            label: '统一管理',
            delayedEcho: '规划研究者在第二章更易结盟。',
          },
        ],
        echoIn: 'ch02.along_river_sleepless',
      },
      unlocksNext: 'ch02.along_river_sleepless',
      routes: {
        main: '巷口探索 → 多方对话 → 选择 → 回声',
        optional: ['店主侧写', '摄影伦理便签'],
        noAnimal: '告示栏文本与语音便签即可推进',
        homeMode: '热线记录 + 街景明信片',
        noCamera: '图库/合成巷口帧',
        badWeather: '室内口述与旧照片对照',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
    {
      id: 'ch02.along_river_sleepless',
      order: 2,
      title: '沿河不眠',
      themeQuestion: '灯光、施工与夜间活动，谁先占用公共河岸？',
      emotionCurve: ['tension', 'empathy', 'doubt'],
      budgetMinutes: { min: 30, max: 45 },
      beats: [
        {
          id: 'c2.open',
          kind: 'open',
          title: '围挡与灯带',
          summary: '白天可走的步道夜里被切段。',
          emotion: 'tension',
          minutes: 8,
        },
        {
          id: 'c2.escalate',
          kind: 'escalate',
          title: '多方冲突',
          summary: '安全、生境、可达性互相不可兼得。',
          emotion: 'tension',
          minutes: 12,
        },
        {
          id: 'c2.turn',
          kind: 'turn',
          title: '没有唯一正解',
          summary: '两个合理方案各有得失。',
          emotion: 'doubt',
          minutes: 8,
        },
        {
          id: 'c2.afterglow',
          kind: 'afterglow',
          title: '雨季预告',
          summary: '传闻将在屋檐下发酵。',
          emotion: 'curiosity',
          minutes: 4,
        },
      ],
      cast: [
        { characterId: 'urban_planner', change: '公开自己的模型假设边界' },
        { characterId: 'archivist', change: '承认夜间口述样本不足' },
        { characterId: 'street_photographer', change: '拒绝夜拍真实动物，改拍灯带与围挡' },
        { characterId: 'journal_aide', change: '强制提示：夜间外出非必须' },
      ],
      exploreNode: {
        id: 'exp.riverfront',
        placeLabel: '滨水区（粗粒度）',
        activity: '日间路线或 Home Mode 便签',
      },
      collectionTrigger: {
        id: 'col.river_notes',
        description: '记录灯光/围挡/通行三类证据中的两类',
        minObservations: 2,
      },
      characterConflict: '加亮路灯 vs 夜行生境；施工安全 vs 无障碍通行。',
      choice: {
        id: 'choice.ch02.light_policy',
        prompt: '河岸夜间灯光你更倾向？',
        options: [
          {
            id: 'safer_brighter',
            label: '优先可达与安全感',
            delayedEcho: '第四章「空白」会少一条夜行线索。',
          },
          {
            id: 'dimmer_habitat',
            label: '优先低干扰',
            delayedEcho: '第三章传闻会引用「你站在生境一侧」。',
          },
        ],
        echoIn: 'ch03.rain_on_eaves',
      },
      unlocksNext: 'ch03.rain_on_eaves',
      routes: {
        main: '日间滨水 → 冲突对话 → 选择',
        optional: ['模拟辩论卡'],
        noAnimal: '灯带/围挡证据即可',
        homeMode: '热线与回忆便签',
        noCamera: '合成明信片',
        badWeather: '室内雨声与旧地图对照',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
    {
      id: 'ch03.rain_on_eaves',
      order: 3,
      title: '雨落屋檐',
      themeQuestion: '当模型、传闻与观察互相矛盾，如何保留判断？',
      emotionCurve: ['doubt', 'tension', 'empathy'],
      budgetMinutes: { min: 30, max: 45 },
      beats: [
        {
          id: 'c3.open',
          kind: 'open',
          title: '屋檐下的三种说法',
          summary: '同一「缺席」被解释为迁移、施工与误传。',
          emotion: 'doubt',
          minutes: 8,
        },
        {
          id: 'c3.escalate',
          kind: 'escalate',
          title: '不可靠叙述',
          summary: '模型置信区间与口述时间线错位。',
          emotion: 'tension',
          minutes: 12,
        },
        {
          id: 'c3.turn',
          kind: 'turn',
          title: '保留空白',
          summary: '玩家选择是否把不确定写进手账正文。',
          emotion: 'agency',
          minutes: 8,
        },
        {
          id: 'c3.afterglow',
          kind: 'afterglow',
          title: '地图空白预告',
          summary: '下一章目标可能根本不出现。',
          emotion: 'absence',
          minutes: 4,
        },
      ],
      cast: [
        { characterId: 'archivist', change: '学会在正文里标注不确定' },
        { characterId: 'urban_planner', change: '公开模型失败案例' },
        { characterId: 'street_photographer', change: '承认照片也会说谎' },
        { characterId: 'journal_aide', change: '提供「缺席证据」模板' },
      ],
      exploreNode: {
        id: 'exp.eaves',
        placeLabel: '雨棚走廊（粗粒度）',
        activity: '对照传闻与观察日志',
      },
      collectionTrigger: {
        id: 'col.unreliable',
        description: '标记至少一条「未证实」线索',
        minObservations: 1,
      },
      characterConflict: '追求闭环叙事 vs 诚实保留空白。',
      choice: {
        id: 'choice.ch03.uncertainty',
        prompt: '不确定的线索如何写入手账？',
        options: [
          {
            id: 'preserve_gaps',
            label: '正文保留空白',
            delayedEcho: '终章结局解锁「多声部空白展」。',
          },
          {
            id: 'provisional_claim',
            label: '写临时结论并标注',
            delayedEcho: '第四章缺席事件会挑战你的临时结论。',
          },
        ],
        echoIn: 'ch04.map_blank',
      },
      unlocksNext: 'ch04.map_blank',
      routes: {
        main: '三种说法 → 对照 → 选择',
        optional: ['模型解释卡'],
        noAnimal: '缺席本身是证据',
        homeMode: '雨声便签 + 旧记录',
        noCamera: '文本/音频为主',
        badWeather: '主路径（雨天正合适）',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
    {
      id: 'ch04.map_blank',
      order: 4,
      title: '地图上的空白',
      themeQuestion: '当目标缺席、季节与城市改造叠加，空白如何被讲述？',
      emotionCurve: ['absence', 'empathy', 'agency'],
      budgetMinutes: { min: 30, max: 45 },
      beats: [
        {
          id: 'c4.open',
          kind: 'open',
          title: '空白格子',
          summary: '地图上一块标注被抹去。',
          emotion: 'absence',
          minutes: 8,
        },
        {
          id: 'c4.escalate',
          kind: 'escalate',
          title: '改造与季节',
          summary: '施工与物候共同解释缺席，仍不充分。',
          emotion: 'doubt',
          minutes: 12,
        },
        {
          id: 'c4.turn',
          kind: 'turn',
          title: '缺席叙事',
          summary: '玩家决定展览如何呈现「什么也没看到」。',
          emotion: 'agency',
          minutes: 8,
        },
        {
          id: 'c4.afterglow',
          kind: 'afterglow',
          title: '策展邀请',
          summary: '终章：把城市交给谁讲。',
          emotion: 'curiosity',
          minutes: 4,
        },
      ],
      cast: [
        { characterId: 'urban_planner', change: '承认规划图也会制造空白' },
        { characterId: 'archivist', change: '提议把空白做成可亲近的展项' },
        { characterId: 'street_photographer', change: '提交一张「故意不拍主体」的作品' },
        { characterId: 'journal_aide', change: '汇总四季选择进入策展清单' },
      ],
      exploreNode: {
        id: 'exp.blank_map',
        placeLabel: '改造区边界（粗粒度）',
        activity: '记录缺席与替代证据',
      },
      collectionTrigger: {
        id: 'col.absence',
        description: '提交一次 fail-forward / 无动物观察记录',
        minObservations: 0,
      },
      characterConflict: '填满地图 vs 让空白可见。',
      choice: {
        id: 'choice.ch04.absence_display',
        prompt: '展览如何呈现缺席？',
        options: [
          {
            id: 'blank_wall',
            label: '留白墙',
            delayedEcho: '终章结局权重倾向「多声部空白」。',
          },
          {
            id: 'proxy_stories',
            label: '用替代叙述填空',
            delayedEcho: '终章更易走向「策展者编排」。',
          },
        ],
        echoIn: 'finale.who_tells_the_city',
      },
      unlocksNext: 'finale.who_tells_the_city',
      routes: {
        main: '空白发现 → 解释竞争 → 选择',
        optional: ['季节物候卡'],
        noAnimal: '缺席路径为主',
        homeMode: '地图标注桌面作业',
        noCamera: '文本/地图 UI',
        badWeather: '完全室内可完成',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
    {
      id: 'finale.who_tells_the_city',
      order: 5,
      title: '把城市交给谁讲',
      themeQuestion: '策展权、叙述权与空白权如何分配？',
      emotionCurve: ['agency', 'catharsis', 'empathy'],
      budgetMinutes: { min: 30, max: 45 },
      beats: [
        {
          id: 'f.open',
          kind: 'open',
          title: '策展清单',
          summary: '汇总四季选择、信任与资料处理方式。',
          emotion: 'agency',
          minutes: 8,
        },
        {
          id: 'f.escalate',
          kind: 'escalate',
          title: '角色立场重逢',
          summary: '四人各自提出展览方案，无唯一正确答案。',
          emotion: 'tension',
          minutes: 12,
        },
        {
          id: 'f.turn',
          kind: 'turn',
          title: '定稿选择',
          summary: '玩家定稿：多声部 / 策展编排 / 空白优先。',
          emotion: 'agency',
          minutes: 8,
        },
        {
          id: 'f.afterglow',
          kind: 'afterglow',
          title: '手账展开放',
          summary: '结局回放关键选择，不按收集率评分。',
          emotion: 'catharsis',
          minutes: 6,
        },
      ],
      cast: [
        { characterId: 'archivist', change: '接受多声部可能比单一权威更诚实' },
        { characterId: 'street_photographer', change: '把署名权让出一部分给被摄者' },
        { characterId: 'urban_planner', change: '把模型失败写进展板' },
        { characterId: 'journal_aide', change: '退到旁白，把话筒交给玩家' },
      ],
      exploreNode: {
        id: 'exp.exhibition',
        placeLabel: '社区展厅（虚构）',
        activity: '布置手账展',
      },
      collectionTrigger: {
        id: 'col.finale_curate',
        description: '选择至少 3 条跨章线索上墙',
        minObservations: 0,
      },
      characterConflict: '权威叙事 vs 多声部 vs 空白权。',
      choice: {
        id: 'choice.finale.curation',
        prompt: '终章展览主叙事？',
        options: [
          {
            id: 'multivocal_show',
            label: '多声部并存',
            delayedEcho: '结局 A：城市被许多人同时讲述',
          },
          {
            id: 'curated_arc',
            label: '策展者编排主线',
            delayedEcho: '结局 B：清晰但承认偏见',
          },
          {
            id: 'blank_first',
            label: '空白优先',
            delayedEcho: '结局 C：缺席成为主角',
          },
        ],
        echoIn: 'finale.who_tells_the_city',
      },
      unlocksNext: null,
      routes: {
        main: '清单 → 四方方案 → 定稿 → 展出',
        optional: ['角色关系回顾'],
        noAnimal: '纯策展',
        homeMode: '桌面布展',
        noCamera: '文本/UI',
        badWeather: '室内',
      },
      passableWithoutRareAnimal: true,
      passableHomeMode: true,
    },
  ],
  dependencyGraph: {
    'prologue.blank_page': [],
    'ch01.alley_echo': ['prologue.blank_page'],
    'ch02.along_river_sleepless': ['ch01.alley_echo'],
    'ch03.rain_on_eaves': ['ch02.along_river_sleepless'],
    'ch04.map_blank': ['ch03.rain_on_eaves'],
    'finale.who_tells_the_city': ['ch04.map_blank'],
  },
  outcomes: [
    {
      id: 'ending.multivocal',
      title: '多声部城市',
      requires: {
        choiceIds: ['multivocal_board', 'preserve_gaps', 'multivocal_show'],
        noteHandling: 'multivocal',
      },
      summary: '展览并置矛盾，不强制和解。',
      independentOfCollectionRate: true,
    },
    {
      id: 'ending.curated',
      title: '策展者的弧',
      requires: {
        choiceIds: ['managed_board', 'curated_arc'],
        noteHandling: 'overwrite',
      },
      summary: '清晰主线，公开偏见声明。',
      independentOfCollectionRate: true,
    },
    {
      id: 'ending.blank_first',
      title: '空白优先',
      requires: {
        choiceIds: ['blank_wall', 'blank_first', 'preserve_gaps'],
        noteHandling: 'preserve_gaps',
      },
      summary: '缺席与未证实成为展核。',
      independentOfCollectionRate: true,
    },
  ],
  contentScale: {
    totalMainMinutes: { min: 165, max: 245 },
    minChoices: 6,
    minDelayedEchoes: 6,
    minHomeModeRoutes: 6,
  },
}

export function listSeasonChapters(): SeasonChapter[] {
  return [...season1Architecture.chapters].sort((a, b) => a.order - b.order)
}

export function getChapter(id: ChapterId): SeasonChapter | undefined {
  return season1Architecture.chapters.find((c) => c.id === id)
}

/** Prerequisites must be completed before chapter is playable */
export function prerequisitesOf(id: ChapterId): ChapterId[] {
  return season1Architecture.dependencyGraph[id] ?? []
}

/** Desktop tabletop: every chapter has required structural slots */
export function validateChapterStructure(ch: SeasonChapter): string[] {
  const errors: string[] = []
  if (ch.beats.length < 4) errors.push(`${ch.id}: need ≥4 beats`)
  const kinds = new Set(ch.beats.map((b) => b.kind))
  for (const k of ['open', 'escalate', 'turn', 'afterglow'] as const) {
    if (!kinds.has(k)) errors.push(`${ch.id}: missing beat ${k}`)
  }
  if (!ch.exploreNode?.placeLabel) errors.push(`${ch.id}: missing explore node`)
  if (!ch.collectionTrigger) errors.push(`${ch.id}: missing collection trigger`)
  if (!ch.characterConflict) errors.push(`${ch.id}: missing character conflict`)
  if (!ch.choice?.options?.length) errors.push(`${ch.id}: missing choice`)
  if (!ch.routes.homeMode || !ch.routes.noCamera || !ch.routes.noAnimal) {
    errors.push(`${ch.id}: missing accessibility routes`)
  }
  if (!ch.passableWithoutRareAnimal || !ch.passableHomeMode) {
    errors.push(`${ch.id}: must pass home/no-rare gates`)
  }
  return errors
}

export function validateSeasonArchitecture(arch: Season1Architecture = season1Architecture): string[] {
  const errors: string[] = []
  if (arch.chapters.length !== 6) errors.push('expected prologue + 4 chapters + finale = 6')
  for (const ch of arch.chapters) {
    errors.push(...validateChapterStructure(ch))
  }
  for (const o of arch.outcomes) {
    if (!o.independentOfCollectionRate) errors.push(`${o.id}: must ignore collection rate`)
  }
  if (arch.outcomes.length < 2) errors.push('need ≥2 endings independent of collection rate')
  // echo chain integrity
  for (const ch of arch.chapters) {
    if (ch.choice.echoIn && !arch.chapters.some((c) => c.id === ch.choice.echoIn)) {
      errors.push(`${ch.id}: echoIn unknown ${ch.choice.echoIn}`)
    }
  }
  return errors
}

/** Low-collection path: only minObservations ≤ 1 chapters required conceptually */
export function chaptersPlayableWithLowCollection(maxObs = 1): ChapterId[] {
  return season1Architecture.chapters
    .filter((c) => c.collectionTrigger.minObservations <= maxObs)
    .map((c) => c.id)
}

export function homeModeRouteCoverage(): { chapterId: ChapterId; route: string }[] {
  return season1Architecture.chapters.map((c) => ({ chapterId: c.id, route: c.routes.homeMode }))
}
