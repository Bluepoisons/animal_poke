interface BattleLogProps {
  lines: string[]
}

export default function BattleLog({ lines }: BattleLogProps) {
  return (
    <div className="ap-battle-log">
      <div>战斗日志</div>
      {lines.map((line, index) => (
        <div key={index}>{line}</div>
      ))}
    </div>
  )
}
