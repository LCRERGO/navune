from ..gamma import normalize


def test_normalize():
    assert normalize(-5) == 0
    assert normalize(3) == 3
