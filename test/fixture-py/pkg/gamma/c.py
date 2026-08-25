from ..alpha import a


class Config:
    def __init__(self, threshold: int = 10) -> None:
        self.threshold = threshold


def normalize(v: int) -> int:
    if v < 0:
        return 0
    return v


def load() -> Config:
    svc = a.Service()
    a.helper(2)
    return Config(threshold=svc.handle(0) or 10)
