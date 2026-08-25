"""pkg/alpha/a.py forms a cycle with beta and gamma."""

from ..beta import b
from ..gamma import c


class Service:
    def handle(self, x: int) -> int:
        if x > 10:
            return b.process(x)
        return c.normalize(x)


def helper(p: int) -> int:
    total = 0
    for i in range(p):
        if i % 2 == 0:
            total += i
    return total
