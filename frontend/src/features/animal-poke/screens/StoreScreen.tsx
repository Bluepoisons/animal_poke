import { useShop } from '../../../shop/useShop'
import { useState } from 'react'
import PageTitle from '../components/PageTitle'
import StoreItemRow from '../components/StoreItemRow'
import { inventoryItems } from '../data/inventory'

interface StoreScreenProps {
  coins: number
  onCoinsChange: (coins: number) => void
  onToast: (message: string) => void
}

export default function StoreScreen({
  coins,
  onCoinsChange,
  onToast,
}: StoreScreenProps) {
  useShop()
  const [checkInDay] = useState(1)
  const [claimedToday, setClaimedToday] = useState(false)
  const rewards = [20, 30, 40, 50, 60, 80, 150]

  const handleCheckIn = () => {
    if (claimedToday) {
      onToast('今日已签到')
      return
    }
    const reward = rewards[checkInDay - 1] ?? rewards[rewards.length - 1]
    onCoinsChange(coins + reward)
    setClaimedToday(true)
    onToast(`签到成功：金币 +${reward}`)
  }

  const handleBuy = (item: (typeof inventoryItems)[0]) => {
    if (coins < item.price) {
      onToast('金币不足')
      return
    }
    onCoinsChange(coins - item.price)
    onToast(`已购买：${item.name}`)
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="商店"
        subtitle="STORE · 手账补给站"
        rightText={`金币 ${coins}`}
        rightTone="yellow"
      />

      <button className="ap-check-card" onClick={handleCheckIn} type="button">
        <h2>7 日签到轨道</h2>
        <div className="ap-reward-row" aria-label="签到奖励">
          {rewards.map((reward, index) => {
            const day = index + 1
            const className =
              day < checkInDay
                ? 'is-done'
                : day === checkInDay
                  ? 'is-current'
                  : ''
            return (
              <span key={day} className={className}>
                {reward}
              </span>
            )
          })}
        </div>
        <p>{claimedToday ? '今天的贴纸已领取' : '第 7 天额外送：玩具球'}</p>
      </button>

      <h2 className="ap-store-section">
        <span className="ap-highlight ap-highlight--blue">道具背包</span>
      </h2>

      <div className="ap-store-list">
        {inventoryItems.map((item) => (
          <StoreItemRow
            key={item.id}
            item={item}
            disabled={coins < item.price}
            onClick={() => handleBuy(item)}
          />
        ))}
      </div>
    </div>
  )
}
