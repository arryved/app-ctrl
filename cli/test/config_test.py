import os
import pytest
import re
import tempfile

from unittest.mock import patch, MagicMock as Mock
from appcontrol.config import (
        merge_config,
        generate_files_from_config,
        create_tarball,
        encode_version_metadata,
        decode_version_metadata,
        push_config_artifact,
)

class TestConfig():
    @patch('appcontrol.config._load_config_file')
    def test_merge_config(self, patched_load_config_file):
        # check that overrides work in the expected precedence order (least -> most)
        patched_load_config_file.side_effect = [
                {"a": "d", "b": "d", "c": "d", "d": "d", "e": "d" },  # d = default
                {"a": "e", "b": "e", "c": "e", "d": "e"},             # e = environment
                {"a": "r", "b": "r", "c": "r"},                       # r = region
                {"a": "v", "b": "v"},                                 # v = variant
        ]
        with tempfile.TemporaryDirectory() as temp_dir:
            result = merge_config(temp_dir, ("test-env", "test-region", "test-variant"))
            assert result == {"a": "v", "b": "v", "c": "r", "d": "e", "e": "d"}

    def test_generate_files_from_config(self):
        # test that files key and control references resolve:
        # 1) extracted from the config (if contents included)
        # 2) copied from the repo dir (if contents absent)
        # 3) control copied over
        example_config = {
                "files": {
                    "path1": None,
                    "path2": "some example file contents in path2",
                }
        }
        with tempfile.TemporaryDirectory() as mock_app_dir:
            os.chdir(mock_app_dir)
            # create path1 file with some contents
            open(f"{mock_app_dir}/path1", "w").write("some example file contents in path1")
            open(f"{mock_app_dir}/control", "w").write("example control script")
            with tempfile.TemporaryDirectory() as temp_dir:
                generate_files_from_config(("test-env", "test-region", "test-variant"), mock_app_dir, temp_dir)
                assert open(f"{temp_dir}/path1", "r").read() == "some example file contents in path1"
                assert open(f"{temp_dir}/path2", "r").read() == "some example file contents in path2"
                assert open(f"{temp_dir}/control", "r").read() == "example control script"

    def test_create_tarball(self):
        with tempfile.TemporaryDirectory() as mock_app_dir:
            os.chdir(mock_app_dir)
            tarball_path = create_tarball(Mock(), mock_app_dir)
            assert os.path.exists(tarball_path)

    def test_encode_decode_version_metadata(self):
        example_version_metadata = {
                "version": "1.2.3" ,
                "application": "my-app",
                "environment": "my-environment",
                "region": "my-region",
                "variant": "my-variant",
                "host": "my-host",
        }

        encoded = encode_version_metadata(example_version_metadata)

        assert encoded == "application=my-app,environment=my-environment,host=my-host,region=my-region,variant=my-variant,version=1.2.3"

        decoded = decode_version_metadata(encoded)

        assert decoded == example_version_metadata


    def test_push_config_artifact(self):
        mock_bucket, mock_blob = Mock(name="mock_bucket"), Mock(name="mock_blob")
        mock_blob.exists.return_value = False
        mock_bucket.blob.return_value = mock_blob
        mock_storage_client = Mock()
        mock_storage_client.bucket.return_value = mock_bucket

        push_config_artifact(mock_storage_client, "/tmp")

        mock_blob.upload_from_filename.assert_called()
