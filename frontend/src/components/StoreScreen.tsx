import React, { useState, useCallback, useMemo } from 'react'
import { useStamina } from '../stamina/useStamina'
import { useShop } from '../shop/useShop'
import { useEconomy } from '../economy/useEconomy'
import { ITEM_DEFS, ITEM_IDS } from '../shop/constants'
import type { ItemId } from '../shop/constants'
import { getTodayString } from '../shop/logic'
import CheckInModal from './CheckInModal'

/** 商店主界面 */
const StoreScreen: React.FC = () => {
  const stamina = useStamina()
  const shop = useShop()
  const economy = useEconomy()
  const [tab, setTab] = useState<'shop' | 'inventory'>('shop')
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'fail' } | null>(null)
  const [checkInModalOpen, setCheckInModalOpen] = useState(false)

  const showToast = useCallback((msg: string, type: 'success' | 'fail' = 'success') => {
    setToast({ msg, type })
    setTimeout(() => setToast(null), 2000)
  }, [])

  const handleBuy = useCallback((itemId: ItemId) => {
    const def = ITEM_DEFS[itemId]
    const result = shop.buyItem(itemId)
    if (result.success) {
      showToast(`购买成功！-${def.price} 🪙`, 'success')
    } else if (result.reason === 'insufficient_gold') {
      showToast('金币不足！', 'fail')
    } else if (result.reason === 'daily_limit_reached') {
      showToast('今日限购已用完！', 'fail')
    }
  }, [shop, showToast])

  const handleUse = useCallback((itemId: ItemId) => {
    const def = ITEM_DEFS[itemId]
    const result = shop.useItem(itemId)
    if (result.success) {
      showToast(`${def.name} 已使用`, 'success')
    } else {
      showToast('道具不存在', 'fail')
    }
  }, [shop, showToast])

  const gold = stamina.state.gold
  const currentStamina = stamina.state.currentStamina
  const maxStamina = stamina.maxStamina

  // 签到状态
  const hasCheckedInToday = shop.state.checkIn.lastCheckInDate === getTodayString()

  // 有道具的背包列表
  const inventoryItems = useMemo(() => {
    return ITEM_IDS.filter(id => (shop.state.inventory[id] ?? 0) > 0)
  }, [shop.state.inventory])

  // 经济统计
  const economyStats = economy.getStats(gold)
  const balance = economy.getBalanceCheck()

  return (
    <div style={styles.container}>
      {/* 经济看板 */}
      <div style={styles.economyDashboard}>
        <div style={styles.dashboardRow}>
          <div style={styles.dashboardCard}>
            <span style={styles.cardLabel}>💰 当前金币</span>
            <span style={styles.cardValue}>{economyStats.currentGold}</span>
          </div>
          <div style={styles.dashboardCard}>
            <span style={styles.cardLabel}>📈 今日产出</span>
            <span style={{ ...styles.cardValue, color: 'var(--success)' }}>+{economyStats.todayEarned}</span>
          </div>
          <div style={styles.dashboardCard}>
            <span style={styles.cardLabel}>📉 今日消耗</span>
            <span style={{ ...styles.cardValue, color: 'var(--warn)' }}>-{economyStats.todaySpent}</span>
          </div>
        </div>
        <div style={styles.balanceBar}>
          <span style={{ color: balance.isHealthy ? 'var(--success)' : 'var(--warn)' }}>
            {balance.status === 'healthy' ? '✅' : '⚠️'} {balance.suggestion}
          </span>
        </div>
      </div>

      {/* 签到入口 */}
      <div style={styles.checkInPanel}>
        <div style={styles.checkInEntry}>
          <span style={styles.checkInTitle}>📅 每日签到</span>
          <span style={styles.streakText}>
            连续 {shop.state.checkIn.streak} 天
          </span>
        </div>
        <button
          className="btn btn-primary"
          style={{
            ...styles.checkInBtn,
            ...(hasCheckedInToday ? styles.disabledBtn : {}),
          }}
          disabled={hasCheckedInToday}
          onClick={() => setCheckInModalOpen(true)}
        >
          {hasCheckedInToday ? '✓ 已签到' : '去签到'}
        </button>
      </div>

      {/* 签到弹窗 */}
      {checkInModalOpen && (
        <CheckInModal onClose={() => setCheckInModalOpen(false)} />
      )}

      {/* Tab 切换 */}
      <div style={styles.tabBar}>
        <button
          className={`btn ${tab === 'shop' ? 'btn-primary' : ''}`}
          style={{ ...styles.tabBtn, ...(tab === 'shop' ? {} : styles.tabBtnInactive) }}
          onClick={() => setTab('shop')}
        >
          🏪 商店
        </button>
        <button
          className={`btn ${tab === 'inventory' ? 'btn-primary' : ''}`}
          style={{ ...styles.tabBtn, ...(tab === 'inventory' ? {} : styles.tabBtnInactive) }}
          onClick={() => setTab('inventory')}
        >
          🎒 背包 ({inventoryItems.length})
        </button>
      </div>

      {/* 内容区 */}
      <div style={styles.content}>
        {tab === 'shop' ? (
          <div style={styles.itemList}>
            {ITEM_IDS.map(itemId => {
              const def = ITEM_DEFS[itemId]
              const canAfford = gold >= def.price
              const dailyPurchased = shop.getDailyPurchaseCount(itemId)
              const dailyRemaining = def.dailyLimit > 0 ? def.dailyLimit - dailyPurchased : 0
              const isDailyLimited = def.dailyLimit > 0
              const isDailySoldOut = isDailyLimited && dailyRemaining <= 0
              // 体力药剂：体力满时禁用
              const isStaminaFull = itemId === 'stamina_potion' && currentStamina >= maxStamina
              const isDisabled = !canAfford || isDailySoldOut || isStaminaFull

              let disabledReason = ''
              if (isStaminaFull) disabledReason = '体力已满'
              else if (isDailySoldOut) disabledReason = '今日已售罄'
              else if (!canAfford) disabledReason = '金币不足'

              return (
                <div key={itemId} style={styles.itemCard}>
                  <div style={styles.itemIcon}>{def.icon}</div>
                  <div style={styles.itemInfo}>
                    <div style={styles.itemNameRow}>
                      <span style={styles.itemName}>{def.name}</span>
                      <span style={styles.itemPrice}>{def.price} 🪙</span>
                    </div>
                    <div style={styles.itemEffect}>{def.effect}</div>
                    {isDailyLimited && (
                      <div style={styles.dailyLimit}>今日剩余 {dailyRemaining}/{def.dailyLimit} 次</div>
                    )}
                  </div>
                  <button
                    className="btn btn-primary"
                    style={{
                      ...styles.buyBtn,
                      ...(isDisabled ? styles.disabledBtn : {}),
                    }}
                    disabled={isDisabled}
                    onClick={() => handleBuy(itemId)}
                  >
                    {isDisabled ? disabledReason : '购买'}
                  </button>
                </div>
              )
            })}
          </div>
        ) : (
          <div style={styles.itemList}>
            {inventoryItems.length === 0 ? (
              <div style={styles.emptyText}>背包空空如也，去商店购买道具吧！</div>
            ) : (
              inventoryItems.map(itemId => {
                const def = ITEM_DEFS[itemId]
                const count = shop.getItemCount(itemId)
                const isCaptureItem = itemId === 'toy_ball' || itemId === 'premium_toy_ball'
                const isBoostActive = isCaptureItem && shop.isCaptureBoostActive()

                return (
                  <div key={itemId} style={styles.itemCard}>
                    <div style={styles.itemIcon}>{def.icon}</div>
                    <div style={styles.itemInfo}>
                      <div style={styles.itemNameRow}>
                        <span style={styles.itemName}>{def.name}</span>
                        <span style={styles.itemCount}>×{count}</span>
                      </div>
                      <div style={styles.itemEffect}>{def.effect}</div>
                      {isBoostActive && (
                        <div style={styles.boostActive}>✓ 已激活</div>
                      )}
                    </div>
                    <button
                      className="btn btn-primary"
                      style={styles.buyBtn}
                      onClick={() => handleUse(itemId)}
                    >
                      使用
                    </button>
                  </div>
                )
              })
            )}
          </div>
        )}
      </div>

      {/* Toast 浮层 */}
      {toast && (
        <div style={{
          ...styles.toast,
          ...(toast.type === 'success' ? styles.toastSuccess : styles.toastFail),
        }}>
          {toast.msg}
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    overflow: 'auto',
    background: 'var(--cream)',
    padding: 12,
    paddingBottom: 20,
    position: 'relative',
  },
  // 经济看板
  economyDashboard: {
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: 12,
    marginBottom: 12,
    boxShadow: 'var(--shadow-card)',
  },
  dashboardRow: {
    display: 'flex',
    gap: 8,
    marginBottom: 8,
  },
  dashboardCard: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 2,
    background: 'var(--cream-dark)',
    borderRadius: 'var(--radius-md)',
    padding: '8px 4px',
  },
  cardLabel: {
    fontSize: 10,
    color: 'var(--ink-3)',
    fontWeight: 600,
  },
  cardValue: {
    fontSize: 16,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  balanceBar: {
    fontSize: 11,
    textAlign: 'center',
    padding: '4px 0',
  },
  // 签到面板
  checkInPanel: {
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: 14,
    marginBottom: 12,
    boxShadow: 'var(--shadow-card)',
  },
  checkInTitle: {
    fontSize: 15,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  checkInEntry: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 10,
  },
  checkInGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(7, 1fr)',
    gap: 4,
    marginBottom: 10,
  },
  checkInCell: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '6px 2px',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--cream-dark)',
    position: 'relative',
    minHeight: 48,
  },
  checkInCellDone: {
    background: 'var(--success)',
    opacity: 0.85,
  },
  checkInCellToday: {
    background: 'var(--orange-100)',
    border: '2px solid var(--orange)',
  },
  checkInCellDay7: {
    border: '2px solid var(--coin)',
  },
  checkInDay: {
    fontSize: 9,
    fontWeight: 700,
    color: 'var(--ink-2)',
  },
  checkInReward: {
    fontSize: 10,
    fontWeight: 600,
    color: 'var(--ink)',
  },
  checkInBonus: {
    fontSize: 10,
    position: 'absolute',
    top: 2,
    right: 2,
  },
  checkInCheck: {
    position: 'absolute',
    top: 2,
    left: 2,
    fontSize: 10,
    color: 'var(--white)',
    fontWeight: 700,
  },
  checkInBtn: {
    width: '100%',
    padding: '8px 0',
    fontSize: 14,
    borderRadius: 'var(--radius-md)',
  },
  streakText: {
    textAlign: 'center',
    fontSize: 11,
    color: 'var(--ink-2)',
    marginTop: 6,
  },
  // Tab 栏
  tabBar: {
    display: 'flex',
    gap: 8,
    marginBottom: 10,
  },
  tabBtn: {
    flex: 1,
    padding: '8px 0',
    fontSize: 13,
    borderRadius: 'var(--radius-md)',
  },
  tabBtnInactive: {
    background: 'var(--white)',
    color: 'var(--ink-2)',
    boxShadow: 'none',
  },
  // 内容区
  content: {
    flex: 1,
  },
  itemList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  // 道具卡片
  itemCard: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: 12,
    boxShadow: 'var(--shadow-card)',
  },
  itemIcon: {
    fontSize: 32,
    flexShrink: 0,
    width: 48,
    height: 48,
    display: 'grid',
    placeItems: 'center',
    background: 'var(--orange-50)',
    borderRadius: 'var(--radius-md)',
  },
  itemInfo: {
    flex: 1,
    minWidth: 0,
  },
  itemNameRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 2,
  },
  itemName: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  itemPrice: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--coin)',
  },
  itemCount: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  itemEffect: {
    fontSize: 11,
    color: 'var(--ink-3)',
  },
  dailyLimit: {
    fontSize: 10,
    color: 'var(--warn)',
    marginTop: 2,
  },
  boostActive: {
    fontSize: 10,
    color: 'var(--success)',
    fontWeight: 600,
    marginTop: 2,
  },
  buyBtn: {
    padding: '6px 14px',
    fontSize: 12,
    borderRadius: 'var(--radius-md)',
    flexShrink: 0,
    minWidth: 56,
  },
  disabledBtn: {
    background: 'var(--ink-3)',
    color: 'var(--white)',
    boxShadow: 'none',
    cursor: 'not-allowed',
    opacity: 0.7,
  },
  emptyText: {
    textAlign: 'center',
    color: 'var(--ink-3)',
    fontSize: 13,
    padding: 40,
  },
  // Toast
  toast: {
    position: 'absolute',
    bottom: 20,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '10px 20px',
    borderRadius: 'var(--radius-md)',
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--white)',
    zIndex: 10,
    boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
    whiteSpace: 'nowrap',
  },
  toastSuccess: {
    background: 'var(--success)',
  },
  toastFail: {
    background: 'var(--warn)',
  },
}

export default React.memo(StoreScreen)
