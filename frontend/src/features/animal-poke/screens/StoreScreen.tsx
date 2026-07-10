import { useCallback, useRef } from 'react'
import PageTitle from '../components/PageTitle'
import StoreItemRow from '../components/StoreItemRow'
import { inventoryItems } from '../data/inventory'
import { useShop } from '../../../shop/useShop'
import { useStamina } from '../../../stamina/useStamina'
import { CHECK_IN_REWARDS, ITEM_DEFS, type ItemId } from '../../../shop/constants'

interface StoreScreenProps {
  /** @deprecated 金币以 StaminaContext 为准 */
  coins?: number
  onCoinsChange?: (coins: number) => void
  onToast: (message: string) => void
}

export default function StoreScreen({ onToast }: StoreScreenProps) {
  const shop = useShop()
  const stamina = useStamina()
  const gold = stamina.state.gold
  const buyLock = useRef(false)
  const checkInLock = useRef(false)

  const checkStatus = shop.getCheckInStatus()
  const day = Math.min(Math.max(checkStatus.todayCycleDay || 1, 1), 7)

  const handleCheckIn = useCallback(() => {
    if (checkInLock.current) return
    checkInLock.current = true
    try {
      const result = shop.checkIn()
      if (!result.success) {
        const reason = 'reason' in result ? String(result.reason) : ''
        onToast(reason === 'already_checked_in' ? '今日已签到' : '签到失败')
        return
      }
      const extra =
        result.rewardItem && ITEM_DEFS[result.rewardItem]
          ? ` · 额外 ${ITEM_DEFS[result.rewardItem].name}`
          : ''
      onToast(`签到成功：金币 +${result.reward}${extra}`)
    } finally {
      queueMicrotask(() => {
        checkInLock.current = false
      })
    }
  }, [shop, onToast])

  const handleBuy = useCallback(
    (itemId: ItemId) => {
      if (buyLock.current) return
      buyLock.current = true
      try {
        const result = shop.buyItem(itemId)
        if (!result.success) {
          if (result.reason === 'insufficient_gold') onToast('金币不足')
          else if (result.reason === 'daily_limit_reached') onToast('已达每日限购')
          else onToast('购买失败')
          return
        }
        const def = ITEM_DEFS[itemId]
        onToast(`已购买：${def.name} · 库存 ${shop.getItemCount(itemId)}`)
      } finally {
        queueMicrotask(() => {
          buyLock.current = false
        })
      }
    },
    [shop, onToast],
  )

  return (
    <div className="ap-screen">
      <PageTitle
        title="商店"
        subtitle="STORE · 手账补给站"
        rightText={`金币 ${gold}`}
        rightTone="yellow"
      />

      <button
        className="ap-check-card"
        onClick={handleCheckIn}
        type="button"
        disabled={checkStatus.hasCheckedInToday}
      >
        <h2>7 日签到轨道</h2>
        <div className="ap-reward-row" aria-label="签到奖励">
          {CHECK_IN_REWARDS.map((reward, index) => {
            const d = index + 1
            const className =
              d < day ? 'is-done' : d === day ? 'is-current' : ''
            return (
              <span key={d} className={className}>
                {reward}
              </span>
            )
          })}
        </div>
        <p>
          {checkStatus.hasCheckedInToday
            ? `今天的贴纸已领取 · 连续 ${checkStatus.currentStreak} 天`
            : `第 ${day} 天可领 · 第 7 天额外送玩具球`}
        </p>
      </button>

      <h2 className="ap-store-section">
        <span className="ap-highlight ap-highlight--blue">道具商店</span>
      </h2>

      <div className="ap-store-list">
        {inventoryItems.map((item) => {
          const id = item.id as ItemId
          const def = ITEM_DEFS[id]
          const dailyUsed = shop.getDailyPurchaseCount(id)
          const limited = def.dailyLimit > 0 && dailyUsed >= def.dailyLimit
          const owned = shop.getItemCount(id)
          return (
            <StoreItemRow
              key={id}
              item={{
                ...item,
                effect: owned > 0 ? `${item.effect} · 持有 ${owned}` : item.effect,
              }}
              disabled={gold < item.price || limited}
              onClick={() => handleBuy(id)}
            />
          )
        })}
      </div>
    </div>
  )
}
