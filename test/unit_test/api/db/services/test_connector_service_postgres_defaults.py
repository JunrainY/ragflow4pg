from types import SimpleNamespace

from common.constants import ConnectorTaskType
from api.db.services import connector_service


class _FakeField:
    def __init__(self, name):
        self.name = name

    def alias(self, _alias):
        return self

    def desc(self):
        return self

    def __eq__(self, other):
        return ("eq", self.name, other)

    def __lt__(self, other):
        return ("lt", self.name, other)


class _FakeQuery:
    def __init__(self):
        self.where_calls = []

    def join(self, *_args, **_kwargs):
        return self

    def where(self, *args, **_kwargs):
        self.where_calls.append(args)
        return self

    def distinct(self):
        return self

    def order_by(self, *_args, **_kwargs):
        return self

    def dicts(self):
        return []


def _fake_sync_logs_model(query):
    field_names = [
        "id",
        "connector_id",
        "task_type",
        "kb_id",
        "update_date",
        "poll_range_start",
        "poll_range_end",
        "new_docs_indexed",
        "total_docs_indexed",
        "error_msg",
        "full_exception_trace",
        "error_count",
        "from_beginning",
        "status",
        "update_time",
    ]
    fields = {name: _FakeField(name) for name in field_names}
    fields["select"] = lambda *_args, **_kwargs: query
    return SimpleNamespace(**fields)


def test_due_connector_tasks_default_to_postgres_interval_sql(monkeypatch):
    monkeypatch.delenv("DB_TYPE", raising=False)
    monkeypatch.setattr(connector_service, "SQL", lambda sql: sql)

    query = _FakeQuery()
    monkeypatch.setattr(connector_service.SyncLogsService, "model", _fake_sync_logs_model(query))

    connector_service.SyncLogsService._list_due_tasks_for_freq(
        ConnectorTaskType.SYNC,
        "refresh_freq",
    )

    interval_conditions = [
        condition
        for where_call in query.where_calls
        for condition in where_call
        if isinstance(condition, tuple) and condition[:2] == ("lt", "update_date")
    ]
    assert interval_conditions
    assert "AT TIME ZONE" in interval_conditions[0][2]
    assert "make_interval" in interval_conditions[0][2]
