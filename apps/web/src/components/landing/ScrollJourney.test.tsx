import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../../i18n/I18nProvider'
import { ScrollJourney } from './ScrollJourney'
import { journeyProgress, journeyStep } from './journeyProgress'

const observers: MockObserver[] = []
class MockObserver {
  callback: IntersectionObserverCallback
  target: Element | null = null
  disconnect = vi.fn()
  constructor(callback: IntersectionObserverCallback) { this.callback = callback; observers.push(this) }
  observe(target: Element) { this.target = target }
  emit(visible: boolean) { this.callback([{ target: this.target, isIntersecting: visible } as IntersectionObserverEntry], this as unknown as IntersectionObserver) }
}
const content = (reducedMotion = false) => <MemoryRouter><I18nProvider><ScrollJourney motionEnabled={!reducedMotion} reducedMotion={reducedMotion} investmentLink="/sites/new"/></I18nProvider></MemoryRouter>
const frames = new Map<number, FrameRequestCallback>()
const originalScrollIntoView = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView')
let frameId = 0
const flushFrames = () => { const pending = [...frames.values()]; frames.clear(); pending.forEach(callback => callback(0)) }

beforeEach(() => {
  observers.length = 0
  frames.clear()
  localStorage.setItem('rbc-language', 'en')
  vi.stubGlobal('IntersectionObserver', MockObserver)
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => { frames.set(++frameId, callback); return frameId }))
  vi.stubGlobal('cancelAnimationFrame', vi.fn((id: number) => frames.delete(id)))
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: vi.fn() })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  if (originalScrollIntoView) Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', originalScrollIntoView)
  else Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView')
})

describe('scroll journey', () => {
  it('clamps progress at both ends and maps it to four ordered steps', () => {
    expect(journeyProgress(900, 3000, 700, 84)).toBe(0)
    expect(journeyProgress(-1066, 3000, 700, 84)).toBe(.5)
    expect(journeyProgress(-3000, 3000, 700, 84)).toBe(1)
    expect(journeyProgress(0, 600, 700, 84)).toBe(0)
    expect([0, .249, .25, .5, .75, 1].map(value => journeyStep(value))).toEqual([0, 0, 1, 2, 3, 3])
  })
  it('changes scenes with scrolling in both directions, stopping work offscreen', () => {
    const { container, unmount } = render(content())
    const section = container.querySelector('#rbc-story')!
    const stage = container.querySelector('.rbc-journey-stage')!
    let top = 84
    Object.defineProperty(section, 'offsetHeight', { value: 3000 })
    Object.defineProperty(stage, 'offsetHeight', { value: 700 })
    vi.spyOn(section, 'getBoundingClientRect').mockImplementation(() => ({ top } as DOMRect))
    act(() => { observers[0].emit(true); flushFrames() })
    expect(screen.getByRole('heading', { level: 3 }).textContent).toBe('Share your location')
    top = -1200
    act(() => { window.dispatchEvent(new Event('scroll')); flushFrames() })
    expect(screen.getByRole('heading', { level: 3 }).textContent).toBe('Shape the project')
    top = 84
    act(() => { window.dispatchEvent(new Event('scroll')); flushFrames() })
    expect(screen.getByRole('heading', { level: 3 }).textContent).toBe('Share your location')
    act(() => observers[0].emit(false))
    const scheduled = vi.mocked(requestAnimationFrame).mock.calls.length
    act(() => window.dispatchEvent(new Event('scroll')))
    expect(requestAnimationFrame).toHaveBeenCalledTimes(scheduled)
    unmount()
    expect(observers[0].disconnect).toHaveBeenCalledOnce()
  })
  it('offers manual navigation without a long pinned scroll in reduced motion', () => {
    const { container } = render(content(true))
    expect(container.querySelector('#rbc-story')?.classList.contains('is-static')).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: /03 Shape the project/ }))
    expect(screen.getByRole('heading', { level: 3 }).textContent).toBe('Shape the project')
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(container.querySelector('video')).toBeNull()
  })
  it('keeps short mobile screens readable with manual scenes instead of a cramped pinned stage', () => {
    vi.mocked(window.matchMedia).mockImplementation(query => ({ matches: query.includes('max-height'), addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList)
    const { container } = render(content())
    expect(container.querySelector('#rbc-story')?.classList.contains('is-static')).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: /04 Imagine a new destination/ }))
    expect(screen.getByRole('heading', { level: 3 }).textContent).toBe('Imagine a new destination')
    expect(window.scrollTo).not.toHaveBeenCalled()
  })
  it('mounts the film only when requested and unmounts it when closed', () => {
    const { container } = render(content(true))
    expect(container.querySelector('video')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Watch the 40-second film' }))
    expect(container.querySelector('details')?.open).toBe(true)
    expect(container.querySelector('video')?.getAttribute('src')).toBeNull()
    const details = container.querySelector('details')!
    act(() => { details.open = false; fireEvent(details, new Event('toggle')) })
    expect(container.querySelector('video')).toBeNull()
  })
  it('returns to the film on every watch click, including when already open', () => {
    const { container } = render(content(true))
    const watch = screen.getByRole('button', { name: 'Watch the 40-second film' })
    fireEvent.click(watch)
    act(flushFrames)
    const details = container.querySelector('details')!
    expect(details.scrollIntoView).toHaveBeenCalledTimes(1)
    fireEvent.click(watch)
    expect(details.scrollIntoView).toHaveBeenCalledTimes(2)
    expect(details.open).toBe(true)
  })
})
