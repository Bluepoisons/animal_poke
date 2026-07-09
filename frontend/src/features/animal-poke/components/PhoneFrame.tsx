import type { ScreenId } from '../data/types'

interface PhoneFrameProps {
  variant: ScreenId
  children: React.ReactNode
}

export default function PhoneFrame({ variant, children }: PhoneFrameProps) {
  return (
    <section className={`ap-phone ap-phone--${variant}`}>{children}</section>
  )
}
