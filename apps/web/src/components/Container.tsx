import type { HTMLAttributes, PropsWithChildren } from 'react'

export interface ContainerProps extends HTMLAttributes<HTMLDivElement> {
  size?: 'default' | 'narrow' | 'wide'
}

export function Container({ children, className = '', size = 'default', ...props }: PropsWithChildren<ContainerProps>) {
  const sizeClass = size === 'narrow' ? 'max-w-5xl' : size === 'wide' ? 'max-w-[1600px]' : 'public-container'
  return (
    <div className={`${sizeClass} ${className}`.trim()} {...props}>
      {children}
    </div>
  )
}
