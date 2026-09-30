export type LiffSDK = {
  init: (options: { liffId: string }) => Promise<void>
  isLoggedIn: () => boolean
  login: (options: { redirectUri: string }) => void
  getIDToken: () => string | null
}

declare global {
  interface Window {
    liff?: LiffSDK
  }
}

export function loadLiffSDK(): Promise<LiffSDK> {
  if (window.liff) return Promise.resolve(window.liff)

  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = 'https://static.line-scdn.net/liff/edge/2/sdk.js'
    script.async = true
    script.onload = () => window.liff ? resolve(window.liff) : reject(new Error('error.LIFF_SDK_LOAD'))
    script.onerror = () => reject(new Error('error.LIFF_CONNECTION'))
    document.head.appendChild(script)
  })
}

export async function loadLiffConfig(errorMessage = 'error.LIFF_CONFIGURATION'): Promise<{ liffId?: string }> {
  const response = await fetch('/api/v1/liff/config')
  if (!response.ok) throw new Error(errorMessage)
  return response.json() as Promise<{ liffId?: string }>
}
