import importlib
import sys
import types
from pathlib import Path


def _stub_module(monkeypatch, name, **attrs):
    module = types.ModuleType(name)
    for key, value in attrs.items():
        setattr(module, key, value)
    monkeypatch.setitem(sys.modules, name, module)
    return module


def test_admin_environment_defaults_to_postgres(monkeypatch):
    monkeypatch.delenv("DB_TYPE", raising=False)
    monkeypatch.syspath_prepend(str(Path.cwd() / "admin" / "server"))
    monkeypatch.delitem(sys.modules, "admin.server.services", raising=False)

    services_pkg = _stub_module(monkeypatch, "api.db.services", UserService=object())
    services_pkg.__path__ = []
    _stub_module(monkeypatch, "api.db.joint_services.user_account_service", create_new_user=lambda *args, **kwargs: None, delete_user_data=lambda *args, **kwargs: None)
    _stub_module(monkeypatch, "api.db.services.canvas_service", UserCanvasService=object())
    _stub_module(monkeypatch, "api.db.services.user_service", TenantService=object(), UserTenantService=object())
    _stub_module(monkeypatch, "api.db.services.knowledgebase_service", KnowledgebaseService=object())
    _stub_module(monkeypatch, "api.db.services.system_settings_service", SystemSettingsService=object())
    _stub_module(monkeypatch, "api.db.services.api_service", APITokenService=object())
    _stub_module(monkeypatch, "api.db.db_models", APIToken=object())
    _stub_module(monkeypatch, "api.utils.crypt", decrypt=lambda value: value)
    _stub_module(monkeypatch, "api.utils.health_utils")
    _stub_module(
        monkeypatch,
        "api.common.exceptions",
        AdminException=type("AdminException", (Exception,), {}),
        UserAlreadyExistsError=type("UserAlreadyExistsError", (Exception,), {}),
        UserNotFoundError=type("UserNotFoundError", (Exception,), {}),
    )
    _stub_module(monkeypatch, "config", SERVICE_CONFIGS=types.SimpleNamespace(configs=[]))

    services = importlib.import_module("admin.server.services")
    envs = {item["env"]: item["value"] for item in services.EnvironmentsMgr.get_all()}

    assert envs["DB_TYPE"] == "postgres"
