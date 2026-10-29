import { ArrowUp } from 'lucide-react'
import { useEffect, useState } from 'react'

export function ScrollToTop({ threshold = 320 }: { threshold?: number }) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    const handleScroll = () => {
      setVisible(window.scrollY > threshold)
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()

    return () => window.removeEventListener('scroll', handleScroll)
  }, [threshold])

  const scrollToTop = () => {
    window.scrollTo({
      top: 0,
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth',
    })
  }

  return (
    <button
      type="button"
      className={`scroll-to-top ${visible ? 'visible' : ''}`}
      onClick={scrollToTop}
      aria-label="เลื่อนขึ้นบนสุด / Scroll to top"
    >
      <ArrowUp size={20} strokeWidth={2.5} />
    </button>
  )
}
