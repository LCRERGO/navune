from ..gamma import c


def process(v: int) -> int:
    if v > 100:
        return c.normalize(v) + 1
    if v > 50:
        return c.normalize(v)
    return c.normalize(v - 1)
