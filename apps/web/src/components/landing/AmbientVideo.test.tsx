import { act, cleanup, fireEvent, render, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AmbientVideo } from './AmbientVideo'
import { prefersLightMedia, useMotionPreference } from './useMotionPreference'

const observers: MockIntersectionObserver[] = []
class MockIntersectionObserver {
  callback: IntersectionObserverCallback
  target: Element | null = null
  constructor(callback: IntersectionObserverCallback) { this.callback = callback; observers.push(this) }
  observe(target: Element) { this.target = target }
  unobserve() {}
  disconnect() {}
  emit(visible: boolean) {
    this.callback([{ target: this.target, isIntersecting: visible } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}

let reduced = false
let mobile = false
const preferenceListeners = new Set<() => void>()
const connectionDescriptor = Object.getOwnPropertyDescriptor(navigator, 'connection')

beforeEach(() => {
  reduced = false
  mobile = false
  preferenceListeners.clear()
  observers.length = 0
  vi.stubGlobal('IntersectionObserver', MockIntersectionObserver)
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({
    matches: query.includes('prefers-reduced-motion') ? reduced : mobile,
    addEventListener: (_event: string, listener: () => void) => preferenceListeners.add(listener),
    removeEventListener: (_event: string, listener: () => void) => preferenceListeners.delete(listener),
  })))
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  Object.defineProperty(navigator, 'connection', { configurable: true, value: { saveData: false, effectiveType: '4g' } })
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  if (connectionDescriptor) Object.defineProperty(navigator, 'connection', connectionDescriptor)
  else Reflect.deleteProperty(navigator, 'connection')
})

describe('AmbientVideo', () => {
  it('defers video loading until visible and uses the mobile rendition on small screens', async () => {
    mobile = true
    const { container } = render(<AmbientVideo enabled/>)
    const video = container.querySelector('video')!
    expect(video.getAttribute('src')).toBeNull()
    act(() => observers[0].emit(false))
    expect(video.getAttribute('src')).toBeNull()
    act(() => observers[0].emit(true))
    expect(video.getAttribute('src')).toBe('/rbc-motion/hero-dynamic-v2-mobile.mp4')
    fireEvent.playing(video)
    expect(video.className).toBe('is-ready')
    await act(async () => {})
  })

  it('does not download autoplay media when motion is disabled or data saver is enabled', async () => {
    const view = render(<AmbientVideo enabled={false}/>)
    const video = view.container.querySelector('video')!
    act(() => observers.at(-1)!.emit(true))
    expect(video.getAttribute('src')).toBeNull()
    Object.defineProperty(navigator, 'connection', { configurable: true, value: { saveData: true } })
    view.rerender(<AmbientVideo enabled/>)
    act(() => observers.at(-1)!.emit(true))
    expect(video.getAttribute('src')).toBeNull()
    expect(video.play).not.toHaveBeenCalled()
    await act(async () => {})
  })

  it('pauses offscreen and restores the image on media failure', async () => {
    const { container } = render(<AmbientVideo enabled/>)
    const video = container.querySelector('video')!
    act(() => observers[0].emit(true))
    fireEvent.playing(video)
    act(() => observers[0].emit(false))
    expect(video.pause).toHaveBeenCalled()
    fireEvent.error(video)
    expect(video.className).toBe('')
    expect(container.querySelector('picture img')?.getAttribute('src')).toBe('/rbc-motion/hero-dynamic-v2.webp')
    await act(async () => {})
  })
})

describe('motion preferences', () => {
  it('responds to a live OS reduced-motion preference change', () => {
    const { result } = renderHook(useMotionPreference)
    expect(result.current).toBe(false)
    act(() => { reduced = true; preferenceListeners.forEach(listener => listener()) })
    expect(result.current).toBe(true)
  })

  it('recognises slow-2g and 2g networks without treating 3g as 2g', () => {
    for (const effectiveType of ['slow-2g', '2g']) {
      Object.defineProperty(navigator, 'connection', { configurable: true, value: { effectiveType } })
      expect(prefersLightMedia()).toBe(true)
    }
    Object.defineProperty(navigator, 'connection', { configurable: true, value: { effectiveType: '3g' } })
    expect(prefersLightMedia()).toBe(false)
  })
})
