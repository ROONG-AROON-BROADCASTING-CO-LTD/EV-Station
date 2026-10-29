export function journeyProgress(top: number, height: number, stageHeight: number, header: number) {
  const distance = height - stageHeight
  return distance > 0 ? Math.max(0, Math.min(1, (header - top) / distance)) : 0
}

export function journeyStep(progress: number, count = 4) {
  return Math.min(count - 1, Math.floor(Math.max(0, progress) * count))
}
