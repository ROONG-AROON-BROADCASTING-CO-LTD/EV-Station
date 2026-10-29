import { useEffect, useRef, useState } from 'react'
import { prefersLightMedia } from './useMotionPreference'

/** The image is always present: rejected autoplay and unavailable video stay usable. */
export function AmbientVideo({ enabled }: { enabled: boolean }) {
  const video = useRef<HTMLVideoElement>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const media = video.current
    if (!media) return
    let visible = false
    let disposed = false
    const sync = () => {
      if (!enabled || !visible || document.hidden || prefersLightMedia()) {
        media.pause()
        return
      }
      if (!media.getAttribute('src')) {
        media.src = window.matchMedia('(max-width: 700px)').matches
          ? '/rbc-motion/hero-dynamic-v2-mobile.mp4' : '/rbc-motion/hero-dynamic-v2.mp4'
      }
      void media.play().then(() => { if (disposed) media.pause() }).catch(() => {})
    }
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting
      sync()
    }, { threshold: .15 })
    observer.observe(media)
    document.addEventListener('visibilitychange', sync)
    return () => {
      disposed = true
      observer.disconnect()
      document.removeEventListener('visibilitychange', sync)
      media.pause()
    }
  }, [enabled])

  return <div className="rbc-cinema-media" aria-hidden="true">
    <picture>
      <source media="(max-width: 700px)" srcSet="/rbc-motion/hero-dynamic-v2-mobile.webp" />
      <img src="/rbc-motion/hero-dynamic-v2.webp" width="1600" height="900" alt="" fetchPriority="high" />
    </picture>
    <video ref={video} className={ready ? 'is-ready' : ''} muted loop playsInline preload="none" onPlaying={() => setReady(true)} onError={() => setReady(false)} />
  </div>
}
