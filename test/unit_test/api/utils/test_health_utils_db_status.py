import os
from unittest.mock import Mock, patch


class TestDatabaseStatus:
    @patch("api.utils.health_utils.DB")
    @patch.dict(os.environ, {"DB_TYPE": "postgres"})
    def test_database_status_uses_postgres_query(self, mock_db):
        cursor = Mock()
        cursor.fetchall.return_value = [("active", 3)]
        mock_db.execute_sql.return_value = cursor

        from api.utils.health_utils import get_database_status

        result = get_database_status()

        assert result["status"] == "alive"
        assert result["message"] == [{"state": "active", "count": 3}]
        mock_db.execute_sql.assert_called_once()
        assert "pg_stat_activity" in mock_db.execute_sql.call_args[0][0]

    @patch("api.utils.health_utils.DB")
    @patch.dict(os.environ, {"DB_TYPE": "mysql"})
    def test_database_status_uses_mysql_query(self, mock_db):
        cursor = Mock()
        cursor.fetchall.return_value = [(1, "root", "localhost", "rag_flow", "Query", 0, "running", "SELECT 1")]
        mock_db.execute_sql.return_value = cursor

        from api.utils.health_utils import get_database_status

        result = get_database_status()

        assert result["status"] == "alive"
        assert result["message"][0]["command"] == "Query"
        mock_db.execute_sql.assert_called_once_with("SHOW PROCESSLIST;")
