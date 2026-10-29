import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../../i18n/I18nProvider'
import { WorkflowFilm } from './WorkflowFilm'

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

const content = (motionEnabled = true) => <MemoryRouter><I18nProvider><WorkflowFilm motionEnabled={motionEnabled} investmentLink="/login?mode=register"/></I18nProvider></MemoryRouter>

beforeEach(() => {
  localStorage.setItem('rbc-language', 'en')
  observers.length = 0
  vi.stubGlobal('IntersectionObserver', MockIntersectionObserver)
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (this: HTMLMediaElement) { this.dispatchEvent(new Event('pause')) })
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
})
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('WorkflowFilm', () => {
  it('does not request the film until explicit playback, and preserves a chapter selected before metadata', async () => {
    const { container } = render(content())
    const video = container.querySelector('video')!
    expect(video.getAttribute('src')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Shape the project/ }))
    expect(video.getAttribute('src')).toBeNull()
    expect(screen.getByText('0:20 / 0:40')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    expect(video.getAttribute('src')).toBe('/rbc-motion/workflow-40s.mp4')
    Object.defineProperty(video, 'readyState', { configurable: true, value: 1 })
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(20.65)
    fireEvent.timeUpdate(video)
    expect(screen.getByRole('button', { name: /Shape the project/ }).getAttribute('aria-pressed')).toBe('true')
    await act(async () => {})
  })

  it('preserves a scrub position chosen before loading the film', async () => {
    const { container } = render(content())
    const video = container.querySelector('video')!
    fireEvent.change(screen.getByRole('slider', { name: 'Video position' }), { target: { value: '32.4' } })
    expect(video.getAttribute('src')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(32.4)
    expect(screen.getByRole('button', { name: /Imagine a new destination/ }).getAttribute('aria-pressed')).toBe('true')
    await act(async () => {})
  })

  it('pauses offscreen without automatically resuming when the visitor returns', async () => {
    const { container } = render(content())
    const video = container.querySelector('video')!
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    fireEvent.playing(video)
    expect(screen.getByRole('button', { name: 'Pause film' })).toBeTruthy()
    act(() => observers.at(-1)!.emit(false))
    expect(screen.getByRole('button', { name: 'Play film' })).toBeTruthy()
    const playsBeforeReturn = vi.mocked(video.play).mock.calls.length
    act(() => observers.at(-1)!.emit(true))
    expect(video.play).toHaveBeenCalledTimes(playsBeforeReturn)
    await act(async () => {})
  })

  it('clears the loading state when motion is paused during buffering', async () => {
    const view = render(content())
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    expect(screen.getByText('Loading film…')).toBeTruthy()
    view.rerender(content(false))
    expect(screen.queryByText('Loading film…')).toBeNull()
    await act(async () => {})
  })

  it('rejects a late playing event after the visitor has scrolled away', async () => {
    const { container } = render(content())
    const video = container.querySelector('video')!
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    act(() => observers.at(-1)!.emit(false))
    fireEvent.playing(video)
    expect(screen.queryByRole('button', { name: 'Pause film' })).toBeNull()
    expect(screen.queryByText('Loading film…')).toBeNull()
    await act(async () => {})
  })

  it('keeps the chapter fallback usable after a media error and retries the same resource', async () => {
    const { container } = render(content())
    const video = container.querySelector('video')!
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    Object.defineProperty(video, 'error', { configurable: true, value: { code: 2 } })
    fireEvent.error(video)
    expect(screen.getByRole('status').textContent).toContain('The film could not load')
    fireEvent.click(screen.getByRole('button', { name: /Understand the potential/ }))
    expect(container.querySelector('.rbc-film-screen img')?.getAttribute('src')).toBe('/rbc-motion/chapter-2.webp')
    fireEvent.click(screen.getByRole('button', { name: 'Play film' }))
    expect(video.load).toHaveBeenCalledOnce()
    expect(screen.queryByRole('status')).toBeNull()
    fireEvent.loadedMetadata(video)
    expect(video.currentTime).toBe(10.65)
    await act(async () => {})
  })
})
