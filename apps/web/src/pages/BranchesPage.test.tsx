import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { BranchesPage } from './BranchesPage'

function Location() { return <output data-testid="location">{useLocation().search}</output> }
const show = (query: string) => render(<MemoryRouter initialEntries={['/branches' + query]}><I18nProvider><BranchesPage/><Location/></I18nProvider></MemoryRouter>)
beforeEach(() => {
  localStorage.setItem('rbc-language', 'en')
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

describe('branch album links', () => {
  it.each([
    ['ayutthaya', 'Ayutthaya', '4'],
    ['phitsanulok', 'Phitsanulok', '5'],
    ['pracha-uthit', 'Pracha Uthit', '5'],
  ])('opens the requested %s album and restores the page when closed', (id, name, count) => {
    show('?album=' + id)
    expect(screen.getByRole('dialog').textContent).toContain(name)
    expect(screen.getByRole('dialog').textContent).toContain('1 / ' + count)
    fireEvent.click(screen.getByRole('button', { name: 'Next image' }))
    expect(screen.getByRole('dialog').textContent).toContain('2 / ' + count)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByTestId('location').textContent).toBe('')
    expect(document.body.style.overflow).not.toBe('hidden')
  })
  it('ignores an unknown album and lets the visitor select a valid branch', () => {
    show('?album=unknown')
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Open album: Phitsanulok branch' }))
    expect(screen.getByRole('dialog').textContent).toContain('Phitsanulok')
    expect(screen.getByTestId('location').textContent).toBe('?album=phitsanulok')
  })
})
