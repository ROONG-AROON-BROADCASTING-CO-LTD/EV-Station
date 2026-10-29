import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useScrollReveal } from './useScrollReveal'

const observers: MockObserver[] = []
class MockObserver {
  callback: IntersectionObserverCallback
  targets: Element[] = []
  disconnect = vi.fn()
  constructor(callback: IntersectionObserverCallback) { this.callback = callback; observers.push(this) }
  observe(target: Element) { this.targets.push(target) }
  emit(target: Element, isIntersecting: boolean, intersectionRatio: number) {
    this.callback([{ target, isIntersecting, intersectionRatio } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}

let scope: HTMLElement
let target: HTMLElement
let root: { current: HTMLElement | null }
const animations: { cancel: ReturnType<typeof vi.fn>; onfinish: (() => void) | null }[] = []
const originalAnimate = Object.getOwnPropertyDescriptor(Element.prototype, 'animate')
const animate = vi.fn(() => {
  const animation = { cancel: vi.fn(), onfinish: null }
  animations.push(animation)
  return animation as unknown as Animation
})

beforeEach(() => {
  observers.length = 0
  animations.length = 0
  animate.mockClear()
  vi.stubGlobal('IntersectionObserver', MockObserver)
  Object.defineProperty(Element.prototype, 'animate', { configurable: true, value: animate })
  scope = document.createElement('main')
  scope.className = 'landing-page'
  scope.innerHTML = '<div class="rbc-experience"><div class="rbc-editorial-heading">Story</div></div><article class="public-value-card">More information</article>'
  document.body.append(scope)
  root = { current: scope.querySelector('.rbc-experience') }
  target = scope.querySelector('.rbc-editorial-heading')!
})

afterEach(() => {
  cleanup()
  scope.remove()
  vi.unstubAllGlobals()
  if (originalAnimate) Object.defineProperty(Element.prototype, 'animate', originalAnimate)
  else Reflect.deleteProperty(Element.prototype, 'animate')
})

describe('repeating scroll reveals', () => {
  it('replays on re-entry but does not restart for small reversals near the threshold', () => {
    renderHook(() => useScrollReveal(root, true))
    const observer = observers[0]
    act(() => observer.emit(target, true, .4))
    expect(animate).toHaveBeenCalledTimes(1)
    act(() => { observer.emit(target, true, .06); observer.emit(target, true, .2) })
    expect(animate).toHaveBeenCalledTimes(1)
    act(() => observer.emit(target, false, 0))
    expect(animations[0].cancel).toHaveBeenCalledOnce()
    act(() => observer.emit(target, true, .4))
    expect(animate).toHaveBeenCalledTimes(2)
  })

  it('does not observe or animate while paused and cancels running motion when disabled', () => {
    const view = renderHook(({ enabled }) => useScrollReveal(root, enabled), { initialProps: { enabled: false } })
    expect(observers).toHaveLength(0)
    view.rerender({ enabled: true })
    act(() => observers[0].emit(target, true, .4))
    view.rerender({ enabled: false })
    expect(observers[0].disconnect).toHaveBeenCalledOnce()
    expect(animations[0].cancel).toHaveBeenCalledOnce()
    expect(observers).toHaveLength(1)
  })

  it('includes lower landing sections and cleans up when navigating away', () => {
    const view = renderHook(() => useScrollReveal(root, true))
    const lowerTarget = scope.querySelector('.public-value-card')!
    expect(observers[0].targets).toContain(lowerTarget)
    act(() => observers[0].emit(lowerTarget, true, .5))
    view.unmount()
    expect(observers[0].disconnect).toHaveBeenCalledOnce()
    expect(animations[0].cancel).toHaveBeenCalledOnce()
  })
})
