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
  const [checkInDay, setCheckInDay] = useState(1)
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
      <PageTitle title="商店 STORE" rightText={`金币 ${coins}`} />
      <button className="ap-check-card" onClick={handleCheckIn} type="button">
        <h2>7 日签到轨道</h2>
        <div className="ap-reward-row">
          {rewards.map((reward, index) => (
            <span
              key={index}
              style={{
                opacity: index + 1 === checkInDay ? 1 : 0.5,
                color: index + 1 < checkInDay ? '#fff8f0' : undefined,
              }}
            >
              {reward}
            </span>
          ))}
        </div>
        <p>第 7 天额外送：玩具球 🎾</p>
      </button>
      <h2 className="ap-store-section">道具背包</h2>
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
