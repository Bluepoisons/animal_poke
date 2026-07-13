import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import type { AnimalRecord } from '../../../db/types'
import { EmptyState, ErrorState, LoadingState } from '../../../components/states'
import AnimalIcon from '../components/AnimalIcon'
import {
  completeAdventure,
  fetchAdventureCompanion,
  fetchAdventureHistory,
  generateAdventure,
  newAdventureOperationId,
  type AdventureChoice,
  type AdventureCompletion,
  type AdventureHistoryItem,
  type AdventureStory,
  type AdventureThemeId,
  type CompanionSnapshot,
} from '../adventureApi'
import { COMPANION_THRESHOLDS } from '../growth'
import { canCaptureSpecies, getCardSpecies, UNKNOWN_SPECIES } from '../../../types'
import {
  chineseClassName,
  chineseDetectedSpeciesName,
  chineseElementName,
  chinesePetSubtitle,
  displayPetName,
} from '../petLocalization'

type Stage = 'camp' | 'generating' | 'exploring' | 'encounter' | 'settling' | 'result'

type Point = { x: number; y: number }

type AdventureScreenProps = {
  onToast: (message: string) => void
  onOpenCollection: () => void
}

const themes: Array<{
  id: AdventureThemeId
  name: string
  kicker: string
  description: string
  icon: string
  target: Point
}> = [
  {
    id: 'mistwood',
    name: '雾灯森径',
    kicker: '森林 · 奇幻',
    description: '沿着萤光苔藓寻找会回应脚步的古老铃声。',
    icon: '✦',
    target: { x: 3, y: 1 },
  },
  {
    id: 'sky_ruins',
    name: '浮空遗迹',
    kicker: '云海 · 谜题',
    description: '登上漂浮石阶，唤醒沉睡在云层之上的星图机关。',
    icon: '◇',
    target: { x: 2, y: 0 },
  },
  {
    id: 'tide_isles',
    name: '潮汐群岛',
    kicker: '海岛 · 精灵',
    description: '跟随月光贝壳铺成的小路，拜访短暂出现的岛屿。',
    icon: '◌',
    target: { x: 3, y: 2 },
  },
]

const choiceIcons: Record<AdventureChoice['id'], string> = {
  courage: '⚔',
  curiosity: '⌕',
  kindness: '♡',
}

const choiceTags: Record<AdventureChoice['id'], string> = {
  courage: '勇气',
  curiosity: '好奇',
  kindness: '温柔',
}

const startPoint: Point = { x: 0, y: 3 }

function animalUUID(record: AnimalRecord): string {
  return record.uuid || record.id
}

function bondTitle(level: number): string {
  if (level >= 7) return '心灵相通'
  if (level >= 4) return '默契搭档'
  if (level >= 2) return '可靠旅伴'
  if (level >= 1) return '初识伙伴'
  return '第一次同行'
}

function bondProgress(snapshot?: CompanionSnapshot): { percent: number; current: number; next: number } {
  const xp = snapshot?.bond_xp ?? 0
  const level = snapshot?.bond_level ?? 0
  const current = COMPANION_THRESHOLDS[level] ?? 0
  const next = COMPANION_THRESHOLDS[level + 1] ?? COMPANION_THRESHOLDS.at(-1) ?? 500
  const percent = next <= current ? 100 : Math.max(0, Math.min(100, ((xp - current) / (next - current)) * 100))
  return { percent, current: xp, next }
}

export default function AdventureScreen({ onToast, onOpenCollection }: AdventureScreenProps) {
  const [pets, setPets] = useState<AnimalRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [themeId, setThemeId] = useState<AdventureThemeId>('mistwood')
  const [stage, setStage] = useState<Stage>('camp')
  const [story, setStory] = useState<AdventureStory | null>(null)
  const [completion, setCompletion] = useState<AdventureCompletion | null>(null)
  const [position, setPosition] = useState<Point>(startPoint)
  const [companion, setCompanion] = useState<CompanionSnapshot | undefined>()
  const [history, setHistory] = useState<AdventureHistoryItem[]>([])
  const [generationError, setGenerationError] = useState<string | null>(null)
  const requestAbort = useRef<AbortController | null>(null)

  const loadPets = useCallback(async () => {
    setLoading(true)
    setLoadError(false)
    try {
      const rows = await AnimalRepository.getUnlocked()
      setPets(rows)
      setSelectedId((current) => current && rows.some((row) => row.id === current) ? current : rows[0]?.id ?? null)
    } catch {
      setLoadError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadPets()
    return () => requestAbort.current?.abort()
  }, [loadPets])

  const selectedPet = useMemo(
    () => pets.find((pet) => pet.id === selectedId) ?? null,
    [pets, selectedId],
  )
  const selectedTheme = themes.find((theme) => theme.id === themeId) ?? themes[0]
  const selectedName = selectedPet ? displayPetName(selectedPet) : ''
  const selectedSpecies = selectedPet ? getCardSpecies(selectedPet) : UNKNOWN_SPECIES
  const canSelectedPetAdventure = selectedPet !== null && canCaptureSpecies(selectedSpecies)
  const bond = bondProgress(companion)

  useEffect(() => {
    if (!selectedPet) {
      setCompanion(undefined)
      setHistory([])
      return
    }
    let active = true
    const uuid = animalUUID(selectedPet)
    void Promise.allSettled([
      fetchAdventureCompanion(uuid),
      fetchAdventureHistory(uuid),
    ]).then(([companionResult, historyResult]) => {
      if (!active) return
      if (companionResult.status === 'fulfilled') setCompanion(companionResult.value)
      else setCompanion(undefined)
      if (historyResult.status === 'fulfilled') setHistory(historyResult.value)
      else setHistory([])
    })
    return () => {
      active = false
    }
  }, [selectedPet])

  const startAdventure = async () => {
    if (!selectedPet || !canSelectedPetAdventure) return
    requestAbort.current?.abort()
    const controller = new AbortController()
    requestAbort.current = controller
    setStage('generating')
    setStory(null)
    setCompletion(null)
    setGenerationError(null)
    setPosition(startPoint)
    try {
      const nextStory = await generateAdventure(
        animalUUID(selectedPet),
        themeId,
        newAdventureOperationId(),
        controller.signal,
      )
      setStory(nextStory)
      setStage('exploring')
    } catch (error) {
      if (controller.signal.aborted) return
      setGenerationError(error instanceof Error ? error.message : 'adventure_generation_failed')
      setStage('camp')
      onToast('中文剧情生成失败，请稍后重试')
    }
  }

  const move = useCallback((dx: number, dy: number) => {
    if (stage !== 'exploring') return
    setPosition((current) => ({
      x: Math.max(0, Math.min(3, current.x + dx)),
      y: Math.max(0, Math.min(3, current.y + dy)),
    }))
  }, [stage])

  useEffect(() => {
    if (stage !== 'exploring') return
    const onKeyDown = (event: KeyboardEvent) => {
      const directions: Record<string, Point> = {
        ArrowUp: { x: 0, y: -1 },
        ArrowDown: { x: 0, y: 1 },
        ArrowLeft: { x: -1, y: 0 },
        ArrowRight: { x: 1, y: 0 },
      }
      const direction = directions[event.key]
      if (!direction) return
      event.preventDefault()
      move(direction.x, direction.y)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [move, stage])

  useEffect(() => {
    if (
      stage === 'exploring' &&
      position.x === selectedTheme.target.x &&
      position.y === selectedTheme.target.y
    ) {
      const timer = window.setTimeout(() => setStage('encounter'), 280)
      return () => window.clearTimeout(timer)
    }
  }, [position, selectedTheme.target.x, selectedTheme.target.y, stage])

  const choose = async (choice: AdventureChoice) => {
    if (!story || stage !== 'encounter') return
    setStage('settling')
    try {
      const result = await completeAdventure(story.adventure_id, choice.id)
      setCompletion(result)
      if (result.companion) setCompanion(result.companion)
      if (selectedPet) {
        void fetchAdventureHistory(animalUUID(selectedPet)).then(setHistory).catch(() => undefined)
      }
      setStage('result')
    } catch {
      setStage('encounter')
      onToast('奇遇结算失败，请重试刚才的选择')
    }
  }

  const returnToCamp = () => {
    setStory(null)
    setCompletion(null)
    setPosition(startPoint)
    setGenerationError(null)
    setStage('camp')
  }

  if (loading) {
    return <div className="ap-screen ap-adventure-screen"><LoadingState title="正在召集你的动物伙伴…" /></div>
  }

  if (loadError) {
    return (
      <div className="ap-screen ap-adventure-screen">
        <ErrorState
          title="动物记录读取失败"
          body="无法确认本次探险的伙伴，请重新读取动物记录。"
          primary={{ label: '重新读取', onClick: () => void loadPets() }}
        />
      </div>
    )
  }

  if (!pets.length) {
    return (
      <div className="ap-screen ap-adventure-screen" data-testid="adventure-empty">
        <div className="ap-adventure-heading">
          <span className="ap-adventure-heading__eyebrow">中文幻想奇遇</span>
          <h1>伙伴远征</h1>
          <p>先在动物记录中拥有一位伙伴，才能开启属于你们的中文幻想剧情。</p>
        </div>
        <EmptyState
          title="营地里还没有伙伴"
          body="完成一次动物记录后，它就能成为你在幻想世界中的探险搭档。"
          primary={{ label: '前往动物记录', onClick: onOpenCollection }}
        />
      </div>
    )
  }

  if ((stage === 'exploring' || stage === 'encounter' || stage === 'settling' || stage === 'result') && story && selectedPet) {
    return (
      <div className={`ap-screen ap-adventure-screen ap-adventure-run is-${story.theme}`} data-testid="adventure-run">
        <header className="ap-adventure-run__header">
          <button type="button" className="ap-adventure-back" onClick={returnToCamp} aria-label="返回探险营地">‹</button>
          <div>
            <span>{story.location}</span>
            <h1>{story.title}</h1>
          </div>
          <span className="ap-adventure-run__chapter">奇遇 01</span>
        </header>

        {stage === 'exploring' && (
          <>
            <section className="ap-adventure-map" aria-label="幻想探险地图">
              {Array.from({ length: 16 }, (_, index) => {
                const x = index % 4
                const y = Math.floor(index / 4)
                const isPlayer = x === position.x && y === position.y
                const isTarget = x === selectedTheme.target.x && y === selectedTheme.target.y
                return (
                  <div
                    className={`ap-adventure-tile ${isTarget ? 'is-target' : ''}`}
                    key={`${x}-${y}`}
                    aria-label={isTarget ? '奇遇地点' : undefined}
                  >
                    {isTarget && !isPlayer && <span className="ap-adventure-event-marker">!</span>}
                    {isPlayer && (
                      <span className="ap-adventure-map-pet" aria-label={`${selectedName} 当前所在位置`}>
                        {selectedPet.photoDataUrl ? (
                          <img src={selectedPet.photoDataUrl} alt="" />
                        ) : (
                          <AnimalIcon species={selectedSpecies} size={32} tone="light" />
                        )}
                      </span>
                    )}
                  </div>
                )
              })}
            </section>

            <section className="ap-rpg-dialogue" aria-live="polite">
              <div className="ap-rpg-dialogue__speaker">旁白</div>
              <p>{story.opening}</p>
              <small>操纵 {selectedName} 前往闪光的「！」位置，触发本次奇遇。</small>
            </section>

            <div className="ap-adventure-controls" aria-label="移动方向键">
              <button type="button" onClick={() => move(0, -1)} aria-label="向上移动">↑</button>
              <button type="button" onClick={() => move(-1, 0)} aria-label="向左移动">←</button>
              <span aria-hidden="true">✦</span>
              <button type="button" onClick={() => move(1, 0)} aria-label="向右移动">→</button>
              <button type="button" onClick={() => move(0, 1)} aria-label="向下移动">↓</button>
            </div>
          </>
        )}

        {(stage === 'encounter' || stage === 'settling') && (
          <section className="ap-adventure-encounter" data-testid="adventure-encounter">
            <div className="ap-adventure-encounter__sigil" aria-hidden="true">!</div>
            <span className="ap-adventure-encounter__eyebrow">奇遇触发</span>
            <h2>{story.encounter_title}</h2>
            <p>{story.encounter}</p>
            <blockquote>{story.companion_line}</blockquote>
            <div className="ap-adventure-choices" aria-label="奇遇选择">
              {story.choices.map((choice) => (
                <button
                  type="button"
                  key={choice.id}
                  disabled={stage === 'settling'}
                  onClick={() => void choose(choice)}
                  data-testid={`adventure-choice-${choice.id}`}
                >
                  <span className="ap-adventure-choice__icon" aria-hidden="true">{choiceIcons[choice.id]}</span>
                  <span>
                    <small>{choiceTags[choice.id]}</small>
                    <b>{choice.label}</b>
                    <em>{choice.description}</em>
                  </span>
                </button>
              ))}
            </div>
            {stage === 'settling' && <div className="ap-adventure-settling">正在让这段回忆落进你们的羁绊里…</div>}
          </section>
        )}

        {stage === 'result' && completion && (
          <section className="ap-adventure-result" data-testid="adventure-result">
            <div className="ap-adventure-result__stars" aria-hidden="true">✦　✧　✦</div>
            <span className="ap-adventure-encounter__eyebrow">奇遇完成</span>
            <h2>你们带着新的默契回来了</h2>
            <p className="ap-adventure-result__outcome">{completion.outcome}</p>
            <div className="ap-adventure-souvenir">
              <span aria-hidden="true">◇</span>
              <div>
                <small>共同回忆</small>
                <b>{completion.souvenir.name}</b>
                <p>{completion.souvenir.description}</p>
              </div>
            </div>
            <div className="ap-adventure-bond-gain">
              <span>羁绊加深</span>
              <b>+{completion.choice.bond_delta ?? 6}</b>
              <small>{bondTitle(companion?.bond_level ?? 0)} · 羁绊 {companion?.bond_xp ?? bond.current}</small>
            </div>
            <button type="button" className="ap-adventure-primary" onClick={returnToCamp}>返回探险营地</button>
          </section>
        )}

        <p className="ap-adventure-fiction-note">{story.disclaimer}</p>
      </div>
    )
  }

  return (
    <div className="ap-screen ap-adventure-screen" data-testid="adventure-screen">
      <div className="ap-adventure-heading">
        <span className="ap-adventure-heading__eyebrow">中文幻想奇遇</span>
        <h1>伙伴远征</h1>
        <p>操纵你的动物伙伴探索幻想地图，每次奇遇都由人工智能依据它的档案生成中文剧情。</p>
      </div>

      <section className="ap-adventure-section" aria-labelledby="adventure-companion-title">
        <div className="ap-adventure-section__title">
          <div><span>01</span><h2 id="adventure-companion-title">选择同行伙伴</h2></div>
          <small>{pets.length} 位伙伴</small>
        </div>
        <div className="ap-adventure-pet-list" role="list">
          {pets.map((pet) => {
            const name = displayPetName(pet)
            const species = getCardSpecies(pet)
            const active = pet.id === selectedId
            return (
              <button
                type="button"
                role="listitem"
                aria-pressed={active}
                className={active ? 'is-active' : ''}
                key={pet.id}
                onClick={() => setSelectedId(pet.id)}
              >
                <span className="ap-adventure-pet-list__avatar">
                  {pet.photoDataUrl ? <img src={pet.photoDataUrl} alt="" /> : <AnimalIcon species={species} size={42} tone="light" />}
                </span>
                <b>{name}</b>
                <small>{chineseDetectedSpeciesName(pet.species, pet.speciesLabelZh)}</small>
              </button>
            )
          })}
        </div>

        {selectedPet && (
          <div className="ap-adventure-companion-card">
            <div className="ap-adventure-companion-card__avatar">
              {selectedPet.photoDataUrl ? (
                <img src={selectedPet.photoDataUrl} alt={`${selectedName} 的伙伴头像`} />
              ) : (
                <AnimalIcon species={selectedSpecies} size={76} tone="light" />
              )}
            </div>
            <div className="ap-adventure-companion-card__body">
              <span className="ap-adventure-companion-card__label">当前伙伴</span>
              <h3>{selectedName}</h3>
              <p>{chinesePetSubtitle(selectedPet)}</p>
              <div className="ap-adventure-bond-line">
                <span>{bondTitle(companion?.bond_level ?? 0)}</span>
                <b>{bond.current} / {bond.next}</b>
              </div>
              <div className="ap-adventure-bond-bar"><span style={{ width: `${bond.percent}%` }} /></div>
            </div>
            <div className="ap-adventure-companion-card__stats" aria-label="伙伴冒险属性">
              <span><small>身份</small><b>{chineseClassName(selectedPet.className)}</b></span>
              <span><small>元素</small><b>{chineseElementName(selectedPet.element)}</b></span>
              <span><small>速度</small><b>{selectedPet.spd ?? '—'}</b></span>
            </div>
          </div>
        )}
      </section>

      {selectedPet && !canSelectedPetAdventure && (
        <div className="ap-adventure-inline-error" role="status">
          需确认物种后才能探险
        </div>
      )}

      <section className="ap-adventure-section" aria-labelledby="adventure-destination-title">
        <div className="ap-adventure-section__title">
          <div><span>02</span><h2 id="adventure-destination-title">决定幻想目的地</h2></div>
          <small>每次剧情不同</small>
        </div>
        <div className="ap-adventure-theme-list">
          {themes.map((theme) => (
            <button
              type="button"
              key={theme.id}
              className={theme.id === themeId ? 'is-active' : ''}
              aria-pressed={theme.id === themeId}
              onClick={() => setThemeId(theme.id)}
            >
              <span className="ap-adventure-theme-list__icon" aria-hidden="true">{theme.icon}</span>
              <span><small>{theme.kicker}</small><b>{theme.name}</b><em>{theme.description}</em></span>
              <i aria-hidden="true">›</i>
            </button>
          ))}
        </div>
      </section>

      {generationError && (
        <div className="ap-adventure-inline-error" role="alert">
          人工智能暂时没有写出合格的中文奇遇。本次不会增加羁绊，请稍后重新生成。
        </div>
      )}

      <button
        type="button"
        className="ap-adventure-primary ap-adventure-primary--launch"
        onClick={() => void startAdventure()}
        disabled={stage === 'generating' || !canSelectedPetAdventure}
        data-testid="adventure-start"
      >
        <span aria-hidden="true">✦</span>
        开始生成这次远征
        <small>{selectedName} · {selectedTheme.name}</small>
      </button>

      {stage === 'generating' && (
        <div className="ap-adventure-generating" role="status" aria-live="polite" data-testid="adventure-generating">
          <span className="ap-adventure-generating__orb" aria-hidden="true">✦</span>
          <div><b>正在生成全中文奇遇</b><p>人工智能正在阅读 {selectedName} 的物种、职业、元素与羁绊档案…</p></div>
        </div>
      )}

      {history.some((item) => item.status === 'completed') && (
        <section className="ap-adventure-memories" aria-labelledby="adventure-memory-title">
          <div className="ap-adventure-section__title">
            <div><span>记</span><h2 id="adventure-memory-title">最近的共同回忆</h2></div>
          </div>
          {history.filter((item) => item.status === 'completed').slice(0, 3).map((item) => (
            <div className="ap-adventure-memory" key={item.adventure_id}>
              <span aria-hidden="true">◇</span>
              <div><b>{item.title}</b><p>{item.souvenir || '未命名纪念物'} · 羁绊 +{item.bond_delta || 6}</p></div>
            </div>
          ))}
        </section>
      )}

      <p className="ap-adventure-fiction-note">剧情由人工智能生成，发生在幻想世界，仅用于虚构玩法，不是现实记录。</p>
    </div>
  )
}
