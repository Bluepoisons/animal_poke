import { useMemo, useRef, useState } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import CaptureProbabilityBar from '../components/CaptureProbabilityBar'
import { useStamina } from '../../../stamina/useStamina'
import { createCaptureSession, settleCapture, BEST_MIN, BEST_MAX } from '../../../capture/session'

interface CaptureScreenProps {
  onToast: (message: string) => void
  species?: 'cat' | 'goose' | 'dog'
}

export default function CaptureScreen({ onToast, species = 'goose' }: CaptureScreenProps) {
  const { currentStamina, consumeStamina } = useStamina()
  const [power] = useState(55)
  const sessionRef = useRef(createCaptureSession({ species, power }))
  const session = sessionRef.current
  const captureRate = useMemo(() => {
    if (power >= BEST_MIN && power <= BEST_MAX) return 0.78
    return 0.45
  }, [power])

  const handleCapture = () => {
    const online = typeof navigator === 'undefined' ? true : navigator.onLine
    const result = settleCapture({
      session: sessionRef.current,
      online,
      stamina: currentStamina,
      consumeStamina: (n) => consumeStamina(n),
    })
    sessionRef.current = result.session
    if (result.ok) {
      onToast(`捕获成功：${result.session.species} 已加入图鉴`)
      return
    }
    if (result.reason === 'already_settled') {
      onToast('本轮已结算')
      return
    }
    if (result.reason === 'offline') {
      onToast('离线无法捕获')
      return
    }
    if (result.reason === 'no_stamina') {
      onToast('体力不足')
      return
    }
    onToast('捕获失败，再试一次')
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="CAPTURE"
        subtitle="点击画面投掷 · 手账捕捉页"
        rightText={`体力 -20 · 余 ${currentStamina}`}
        rightTone="pink"
      />

      <div
        className="ap-capture-stage"
        onClick={handleCapture}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault()
            handleCapture()
          }
        }}
        role="button"
        tabIndex={0}
      >
        <AnimalIcon species={session.species} size={120} />
        <CaptureProbabilityBar rate={captureRate} power={power} bestMin={BEST_MIN} bestMax={BEST_MAX} />
      </div>
    </div>
  )
}
