export interface ConcurrencyLimiter {
  run<Result>(operation: () => Promise<Result>): Promise<Result>
}

export function createConcurrencyLimiter(maximumConcurrency: number): ConcurrencyLimiter {
  if (!Number.isInteger(maximumConcurrency) || maximumConcurrency < 1) {
    throw new RangeError('maximumConcurrency must be a positive integer')
  }

  let activeOperationCount = 0
  const pendingResolvers: Array<() => void> = []

  const acquireSlot = async (): Promise<void> => {
    if (activeOperationCount < maximumConcurrency) {
      activeOperationCount += 1
      return
    }

    await new Promise<void>((resolve) => {
      pendingResolvers.push(resolve)
    })
  }

  const releaseSlot = (): void => {
    const resolveNextOperation = pendingResolvers.shift()
    if (resolveNextOperation) {
      resolveNextOperation()
      return
    }
    activeOperationCount -= 1
  }

  return {
    async run<Result>(operation: () => Promise<Result>): Promise<Result> {
      await acquireSlot()
      try {
        return await operation()
      } finally {
        releaseSlot()
      }
    }
  }
}
