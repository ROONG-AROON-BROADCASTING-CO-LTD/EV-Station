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
    let offset = 0
    const centre = 84 + (window.innerHeight - 84) / 2
    container.querySelectorAll('.rbc-journey-chapter').forEach((row, index) => {
      vi.spyOn(row, 'getBoundingClientRect').mockImplementation(() => ({ top: centre - 150 + index * 500 - offset, height: 300 } as DOMRect))
    })
    act(() => { observers[0].emit(true); flushFrames() })
    expect(screen.getByRole('button', { name: 'Share your location' }).getAttribute('aria-pressed')).toBe('true')
    offset = 1000
    act(() => { window.dispatchEvent(new Event('scroll')); flushFrames() })
    expect(screen.getByRole('button', { name: 'Shape the project' }).getAttribute('aria-pressed')).toBe('true')
    expect(container.querySelector('.rbc-journey-images .is-current')?.getAttribute('src')).toBe('/rbc-motion/chapter-3.webp')
    offset = 0
    act(() => { window.dispatchEvent(new Event('scroll')); flushFrames() })
    expect(screen.getByRole('button', { name: 'Share your location' }).getAttribute('aria-pressed')).toBe('true')
    expect(container.querySelector('.rbc-journey-images .is-current')?.getAttribute('src')).toBe('/rbc-motion/chapter-1.webp')
    act(() => observers[0].emit(false))
    const scheduled = vi.mocked(requestAnimationFrame).mock.calls.length
    act(() => window.dispatchEvent(new Event('scroll')))
    expect(requestAnimationFrame).toHaveBeenCalledTimes(scheduled)
    unmount()
    expect(observers[0].disconnect).toHaveBeenCalledOnce()
  })
  it('keeps all chapters readable and uses instant navigation in reduced motion', () => {
    const { container } = render(content(true))
    expect(container.querySelector('#rbc-story')?.classList.contains('is-quiet')).toBe(true)
    expect(screen.getAllByRole('heading', { level: 3 })).toHaveLength(4)
    fireEvent.click(screen.getByRole('button', { name: 'Shape the project' }))
    expect(screen.getByRole('button', { name: 'Shape the project' }).getAttribute('aria-pressed')).toBe('true')
    expect(container.querySelector('#rbc-story-step-3')?.scrollIntoView).toHaveBeenCalledWith({ behavior: 'instant', block: 'center' })
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(container.querySelector('video')).toBeNull()
  })
  it('pairs every chapter with its own inline image for the mobile layout', () => {
    const { container } = render(content())
    expect(container.querySelectorAll('.rbc-journey-mobile-image')).toHaveLength(4)
    container.querySelectorAll('.rbc-journey-chapter').forEach((row, index) => {
      expect(row.querySelector('img')?.getAttribute('src')).toBe(`/rbc-motion/chapter-${index + 1}.webp`)
      expect(row.querySelector('p')?.textContent).not.toBe('')
    })
    fireEvent.click(screen.getByRole('button', { name: 'Imagine a new destination' }))
    expect(container.querySelector('#rbc-story-step-4')?.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
  })
  it('does not render the removed concept-film disclosure or its trigger', () => {
    const { container } = render(content(true))
    expect(container.querySelector('video')).toBeNull()
    expect(container.querySelector('.rbc-journey-film')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Watch the 40-second film' })).toBeNull()
    expect(screen.queryByText('Watch the 40-second concept film')).toBeNull()
  })
  it('skips directly to the next section after the film is removed', () => {
    render(<>{content(true)}<section aria-label="Next section"/></>)
    fireEvent.click(screen.getByRole('button', { name: 'Skip this story' }))
    expect(screen.getByRole('region', { name: 'Next section' }).scrollIntoView).toHaveBeenCalledWith({ behavior: 'instant', block: 'start' })
  })
})
