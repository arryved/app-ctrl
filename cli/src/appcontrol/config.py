#
# Arryved Deploy 2.0 config bundler
#
# config schema: https://docs.google.com/document/d/1ehL0et5j3CNMBz76nRI04P0lQukMFAKQYpq-VDpc90Q/edit#bookmark=id.tg01t5d5ggnw
# config example https://docs.google.com/document/d/1ehL0et5j3CNMBz76nRI04P0lQukMFAKQYpq-VDpc90Q/edit#bookmark=id.solt2q7d3if8
#
import click
import click_spinner
import os
import subprocess
import tarfile
import tempfile
import warnings
import yaml

from deepmerge import always_merger
from google.cloud import storage
from urllib.parse import quote, unquote

from appcontrol.common import constants, working_directory


warnings.filterwarnings("ignore")
APP_ROOT = os.environ.get("APP_ROOT", ".")


def _copy_file(source_path, target_path):
    with open(source_path, "rb") as source_file:
        with open(target_path, "wb") as target_file:
            while True:
                chunk = source_file.read(8192)  # read in 8K chunks
                if not chunk:
                    break
                target_file.write(chunk)


def _dump_config_file(config, path):
    with open(path, "w+") as f:
        yaml.dump(config, stream=f)


def _dump_file(contents, path):
    try:
        with open(path, "w+") as f:
            f.write(contents)
    except Exception as e:
        raise TypeError(f"error writing empty contents to file={path}, err={e}")


def _load_config_file(path):
    with open(path) as f:
        return yaml.safe_load(f.read())


def _make_subdirs(paths, temp_dir):
    for path in paths:
        target_path = os.path.join(temp_dir, path)
        dir_name = os.path.dirname(target_path)
        os.makedirs(dir_name, exist_ok=True)


def merge_config(directory, config_spec):
    """ merge the config per precedence rules for the requested (env, region, variant, host)
        - defaults
        - env/[environment].yaml (if specified)
        - region/[region].yaml (if specified)
        - variant/[variant].yaml (if specified)
        - host/[host].yaml (if specified)
    """
    environment, region, variant, host = config_spec
    directory = f"{directory}/config"

    # load defaults first
    result = _load_config_file(f"{directory}/defaults.yaml")
    # merge environment
    if environment:
        result = always_merger.merge(result, _load_config_file(f"{directory}/env/{environment}.yaml"))
    # merge region
    if region:
        result = always_merger.merge(result, _load_config_file(f"{directory}/region/{region}.yaml"))
    # merge variant
    if variant:
        result = always_merger.merge(result, _load_config_file(f"{directory}/variant/{variant}.yaml"))
    # merge host
    if host:
        result = always_merger.merge(result, _load_config_file(f"{directory}/host/{host}.yaml"))
    return result


def generate_files_from_config(config, config_spec, directory, temp_dir):
    """ generate any files whose contents are in the config, including
        any embedded files; verifies on-disk files are present
    """
    files = config.get("files", {})

    # for all files under the files key, create required subdirs in temp_dir
    _make_subdirs(files.keys(), temp_dir)

    for path, contents in config.get("files", {}).items():
        file_exists_readable = os.path.exists(path) and os.path.isfile(path) and os.access(path, os.R_OK)
        temp_path = f"{temp_dir}/{path}"

        if contents:
            # when value present, dump contents to file in temp_dir
            _dump_file(contents, temp_path)
            click.echo(click.style(f"dumping file at {path}", fg="blue"))
            if file_exists_readable:
                # warn if a file is being overridden
                click.echo(click.style(f"warning: content in config is taking precedence over existing file at {path}", fg="yellow"), err=True)
        else:
            # when value absent, verify that file exists on disk, and fail if it doesn't
            if not file_exists_readable:
                msg = f"expected file at {path} is missing"
                click.echo(click.style(msg, fg="red"), err=True)
                raise ValueError(msg)
            # copy the existing file to the temp_path
            click.echo(click.style(f"copying file at {path}", fg="blue"))
            _copy_file(path, temp_path)

        # copy the control file as well
        _copy_file(f"{directory}/control.py", f"{temp_dir}/control.py")

    # dump merged config file as config.yaml for bundling
    path = f"{temp_dir}/config.yaml"
    _dump_config_file(config, path)


def create_tarball(version_metadata, temp_dir):
    # create an archive to add all the temp_dir files, tagged to the config_spec & version/hash
    with working_directory(temp_dir) as cwd_relative:
        tarfile_path = f"{temp_dir}/config-{version_metadata}.tar.gz"
        with tarfile.open(tarfile_path, "w:gz") as tar:
            # recursively list all files in tree
            for dirpath, dirnames, filenames in os.walk(cwd_relative):
                for filename in filenames:
                    full_path = os.path.join(dirpath, filename)
                    # add to archive
                    tar.add(full_path)
            tar.close()
            return tarfile_path


def _git_hash():
    try:
        return subprocess.check_output(
                [
                    "git",
                    "rev-parse",
                    "--short",
                    "HEAD",
                ],
                universal_newlines=True
        ).strip()
    except subprocess.CalledProcessError:
        print("failed to get current hash for this directory, it's possible this isn't a git repo")
        raise


def encode_version_metadata(metadata_map):
    """
    converts metadata into a string suitable for tagging/naming bundles
    """
    keys = sorted(metadata_map.keys())
    encoded_pairs = [f"{k}={quote(str(metadata_map[k]))}" for k in keys]
    filename = ",".join(encoded_pairs)
    return filename


def decode_version_metadata(filename):
    """
    decodes bundle/tag metadate back into a metadata map
    """
    pairs = filename.split(",")
    metadata_map = {}
    for pair in pairs:
        k, v = pair.split("=")
        metadata_map[k] = unquote(v)
    return metadata_map


def _generate_version_metadata(config_spec, config_map):
    environment, region, host, variant = config_spec
    app = config_map["name"]
    version = ".".join([e for e in config_map["version"].split(".") if e != "*"])
    githash = _git_hash()
    return encode_version_metadata({
        "app": app,
        "environment": environment,
        "hash": githash,
        "host": host,
        "region": region,
        "variant": variant,
        "version": version,
    })


def push_config_artifact(storage_client, source_path):
    repo_name = constants["repo_name"]
    bucket = storage_client.bucket(repo_name)
    destination_path = os.path.basename(source_path)

    blob = bucket.blob(destination_path)
    if blob.exists():
        msg = f"config name={destination_path} already exists in repo"
        raise ValueError(msg)

    file_name = os.path.basename(source_path)
    blob.upload_from_filename(source_path)
    click.echo(click.style(f"Config bundle {file_name} uploaded to repo bucket={repo_name}", fg="green"))


@click.command()
@click.option('-e', '--environment', required=False, default=None)
@click.option('-r', '--region', required=False, default=None)
@click.option('-v', '--variant', required=False, default=None)
@click.option('-h', '--host', required=False, default=None)
def config(environment, region, variant, host):
    click.echo(click.style(f"Building config for environment={environment},region={region},variant={variant},host={host}...", fg="green"))
    arryved_dir = "./.arryved"
    config_spec = (environment, region, variant, host)

    with click_spinner.spinner():
        try:
            merged = merge_config(arryved_dir, config_spec)
            version_metadata = _generate_version_metadata(config_spec, merged)
            with tempfile.TemporaryDirectory() as temp_dir:
                generate_files_from_config(merged, config_spec, arryved_dir, temp_dir)
                tarball_path = create_tarball(version_metadata, temp_dir)
                push_config_artifact(storage.Client(), tarball_path)
        except Exception as e:
            click.echo(click.style(f"Server experienced an error: {e}", fg="red"), err=True)
            exit(1)
