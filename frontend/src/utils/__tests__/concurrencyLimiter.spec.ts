import { describe, expect, it } from 'vitest'

import { createConcurrencyLimiter } from '../concurrencyLimiter'

function createDeferred<Value>() {
  let resolvePromise!: (value: Value | PromiseLike<Value>) => void
  let rejectPromise!: (reason?: unknown) => void
  const promise = new Promise<Value>((resolve, reject) => {
    resolvePromise = resolve
    rejectPromise = reject
  })
  return { promise, resolve: resolvePromise, reject: rejectPromise }
}

async function waitForCondition(condition: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (condition()) return
    await Promise.resolve()
  }
  throw new Error('condition was not met before the microtask deadline')
}

describe('createConcurrencyLimiter', () => {
  it('never runs more than the configured number of operations', async () => {
    const limiter = createConcurrencyLimiter(2)
    const deferredOperations = Array.from({ length: 5 }, () => createDeferred<void>())
    let activeOperationCount = 0
    let maximumObservedConcurrency = 0
    let startedOperationCount = 0

    const operations = deferredOperations.map((deferredOperation) => {
      return limiter.run(async () => {
        startedOperationCount += 1
        activeOperationCount += 1
        maximumObservedConcurrency = Math.max(maximumObservedConcurrency, activeOperationCount)
        await deferredOperation.promise
        activeOperationCount -= 1
      })
    })

    await waitForCondition(() => activeOperationCount === 2)
    expect(activeOperationCount).toBe(2)

    deferredOperations[0].resolve()
    await waitForCondition(() => startedOperationCount === 3)
    expect(activeOperationCount).toBe(2)

    deferredOperations[1].resolve()
    deferredOperations[2].resolve()
    await Promise.resolve()
    await Promise.resolve()

    deferredOperations[3].resolve()
    deferredOperations[4].resolve()
    await Promise.all(operations)

    expect(maximumObservedConcurrency).toBe(2)
    expect(activeOperationCount).toBe(0)
  })

  it('releases a slot when an operation rejects', async () => {
    const limiter = createConcurrencyLimiter(1)
    const firstOperation = createDeferred<void>()
    let secondOperationStarted = false

    const firstResult = limiter.run(() => firstOperation.promise)
    const secondResult = limiter.run(async () => {
      secondOperationStarted = true
      return 'completed'
    })

    await Promise.resolve()
    expect(secondOperationStarted).toBe(false)

    firstOperation.reject(new Error('expected failure'))
    await expect(firstResult).rejects.toThrow('expected failure')
    await expect(secondResult).resolves.toBe('completed')
  })

  it('rejects invalid concurrency limits', () => {
    expect(() => createConcurrencyLimiter(0)).toThrow(RangeError)
    expect(() => createConcurrencyLimiter(1.5)).toThrow(RangeError)
  })
})
