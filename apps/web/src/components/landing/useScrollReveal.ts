import { useEffect, type RefObject } from 'react'

const revealTargets = '.rbc-presenter-copy, .rbc-editorial-heading, .rbc-destination-stage, .public-market-heading, .public-about-copy, .public-about-roles, .public-value-card, .rbc-closing-heading'

/** Re-arm only after fully leaving the viewport, so small scroll reversals do not flicker. */
export function useScrollReveal(root: RefObject<HTMLElement | null>, enabled: boolean) {
  useEffect(() => {
    if (!enabled || !root.current) return
    const scope = root.current.closest('.landing-page') ?? root.current
    const visible = new Set<Element>()
    const animations = new Map<Element, Animation>()
    const observer = new IntersectionObserver(entries => {
      entries.forEach(entry => {
        if (!entry.isIntersecting) {
          visible.delete(entry.target)
          animations.get(entry.target)?.cancel()
          animations.delete(entry.target)
          return
        }
        if (entry.intersectionRatio < .12 || visible.has(entry.target)) return
        visible.add(entry.target)
        const animation = entry.target.animate(
          [{ opacity: .55, transform: 'translateY(24px)' }, { opacity: 1, transform: 'translateY(0)' }],
          { duration: 650, easing: 'cubic-bezier(.16,1,.3,1)' },
        )
        animations.set(entry.target, animation)
        animation.onfinish = () => {
          if (animations.get(entry.target) === animation) animations.delete(entry.target)
        }
      })
    }, { threshold: [0, .12] })
    scope.querySelectorAll(revealTargets).forEach(target => observer.observe(target))
    return () => {
      observer.disconnect()
      animations.forEach(animation => animation.cancel())
    }
  }, [root, enabled])
}
