import os
import pytest
import re
import tempfile

from unittest.mock import patch, MagicMock as Mock
from appcontrol.status import status
from click.testing import CliRunner


class TestStatus():
    def test_status(self):
        runner = CliRunner()
        #result = runner.invoke(status, ['-e', 'stg'])
        #assert 'somethings' in result.output
        #assert result.exit_code == 0
