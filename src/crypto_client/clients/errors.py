class DeribitError(Exception):
    pass


class TransientDeribitError(DeribitError):
    """Temporary error. Safe to retry."""

    pass


class PermanentDeribitError(DeribitError):
    """Permanent error. Retrying won't help."""

    pass
