"""Offline worker regression tests; no database or remote requests are made."""
import io
import json
import tempfile
import unittest
import urllib.error
from pathlib import Path
from unittest.mock import MagicMock, patch

import update_roads


class RoadUpdaterTests(unittest.TestCase):
    def test_current_dataset_skips_download_and_bounds_database_wait(self):
        connection = MagicMock()
        connection.execute.return_value.fetchone.return_value = (False,)
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(update_roads, 'TARGET', Path(directory) / 'roads.pbf'), \
                patch.dict(update_roads.os.environ, {'ROAD_DATABASE_URL': 'test-only'}), \
                patch.object(update_roads.psycopg, 'connect') as connect, \
                patch.object(update_roads.urllib.request, 'urlopen') as download:
            connect.return_value.__enter__.return_value = connection
            update_roads.refresh()
            connect.assert_called_once_with('test-only', connect_timeout=10)
            download.assert_not_called()

    def test_failure_retries_before_six_hours_and_resets_after_success(self):
        # Interrupt only after observing failure, failure, success and failure.
        with patch.object(update_roads, 'refresh', side_effect=[
                OSError('secret must not be logged'), OSError('secret'), None, OSError('secret')]), \
                patch.object(update_roads.time, 'sleep', side_effect=[None, None, None, KeyboardInterrupt]) as sleep, \
                patch.object(update_roads.Path, 'touch'), \
                patch('sys.stdout', new_callable=io.StringIO) as output:
            with self.assertRaises(KeyboardInterrupt):
                update_roads.run_worker()
            self.assertEqual([call.args[0] for call in sleep.call_args_list], [300, 600, 21600, 300])
            self.assertNotIn('secret', output.getvalue())

    def test_repeated_failures_have_bounded_backoff(self):
        self.assertEqual([update_roads.retry_delay(n) for n in (1, 2, 3, 4, 5, 100)],
                         [300, 600, 1200, 2400, 3600, 3600])

    def test_failure_diagnostics_reveal_stage_and_status_without_url_or_credentials(self):
        cause = urllib.error.HTTPError('https://user:secret@example.invalid', 503,
                                       'password=secret', None, None)
        event = update_roads.failure_event(update_roads.RefreshFailure('download', cause), 300)
        self.assertEqual(event['stage'], 'download')
        self.assertEqual(event['http_status'], 503)
        self.assertEqual(event['retry_after_seconds'], 300)
        self.assertNotIn('secret', json.dumps(event))
        self.assertNotIn('example.invalid', json.dumps(event))

    def test_database_failure_keeps_phase_without_exposing_dsn(self):
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(update_roads, 'TARGET', Path(directory) / 'roads.pbf'), \
                patch.dict(update_roads.os.environ, {'ROAD_DATABASE_URL': 'test-only'}), \
                patch.object(update_roads.psycopg, 'connect', side_effect=OSError('password=secret')):
            with self.assertRaises(update_roads.RefreshFailure) as failure:
                update_roads.refresh()
        event = update_roads.failure_event(failure.exception, 300)
        self.assertEqual(event['stage'], 'database')
        self.assertNotIn('secret', json.dumps(event))


if __name__ == '__main__':
    unittest.main()
