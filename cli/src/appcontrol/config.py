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
import shutil
import yaml

from deepmerge import Merger
from google.cloud import storage
from urllib.parse import quote, unquote

from appcontrol.common import constants, working_directory


warnings.filterwarnings("ignore")
APP_ROOT = os.environ.get("APP_ROOT", ".")
DELETE_MARKER = object()


def _delete_marker_constructor(loader, node):
    return DELETE_MARKER


yaml.SafeLoader.add_constructor('!DELETE', _delete_marker_constructor)


def _custom_override_with_dupe_warning(config, path, base, override):
    if isinstance(base, dict) and isinstance(override, dict):
        # override while checking for dupes
        for key, value in override.items():
            if key in base:
                if base[key] == value:
                    click.echo(click.style(f"warning: '{key}' in override has the same value in base and override: {value}", fg="yellow"), err=True)
                if value is DELETE_MARKER:
                    del base[key]
                else:
                    base[key] = _custom_override_with_dupe_warning(config, path + [str(key)], base[key], value)
            else:
                if value is not DELETE_MARKER:
                    base[key] = value

        return base
    else:
        return override


def _override_merger():
    return Merger(
        [(dict, _custom_override_with_dupe_warning)],
        ["override"],
        ["override"],
    )


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


def merge_config(directory, config_spec) -> dict:
    """ merge the config per precedence rules for the requested (env, region, variant)
        - defaults
        - env/[environment].yaml (if specified)
        - region/[region].yaml (if specified)
        - variant/[variant].yaml (if specified)
    """
    environment, region, variant = config_spec
    directory = f"{directory}/config"

    # load defaults first
    result = _load_config_file(f"{directory}/defaults.yaml")
    # merge environment
    if environment:
        result = _override_merger().merge(result, _load_config_file(f"{directory}/env/{environment}.yaml"))
    # merge region
    if region:
        result = _override_merger().merge(result, _load_config_file(f"{directory}/region/{region}.yaml"))
    # merge variant
    if variant:
        result = _override_merger().merge(result, _load_config_file(f"{directory}/variant/{variant}.yaml"))
    return result


def _get_all_file_refs_from_configs(config_dir):
    """ walk through all yamls and get file refs (those with keys but no values) """
    files = {}
    for root, dirs, config_files in os.walk(config_dir):
        for file in config_files:
            if file.endswith(".yaml"):
                # Get the full path of the file
                file_path = os.path.join(root, file)
                # Open the file and load the yaml data
                cfg = _load_config_file(file_path)
                # Get the path keys in the "files" object and add them to the list if they have no inlined value
                cfg_files = cfg.get("files", {})
                for path, value in cfg_files.items():
                    if not value:
                        files[path] = value
    return files


#################################################################
# TODO DRY - this is copypasty push this helper into arpy-config
#
def build_jvm_args_non_standard_unstable_list(args):
    result = []
    for k, v in args.items():
        arg = f"-XX:{k}={v}"
        if type(v) is bool:
            sign = "+" if v else "-"
            arg = f"-XX:{sign}{k}"
        result.append(arg)
    return result


def build_jvm_args_non_standard_list(args):
    result = []
    for k, v in args.items():
        arg = f"-X{k}{v}"
        result.append(arg)
    return result


def build_jvm_args_properties_list(args):
    result = []
    for k, v in args.items():
        arg = f"-D{k}={v}"
        result.append(arg)
    return result


def _simulate_java_cmd(cfg):
    # TODO DRY - this is copypasty push this helper into arpy-config
    java_cfg = cfg["java"]

    jvm_args_app = java_cfg.get("jvm_args_app", {})
    jvm_args_app_list = [f"--{k} {v}" for k, v in jvm_args_app.items()]

    jvm_args_non_standard = java_cfg.get("jvm_args_non_standard", {})
    jvm_args_non_standard_list = build_jvm_args_non_standard_list(jvm_args_non_standard)

    jvm_args_non_standard_unstable = java_cfg.get("jvm_args_non_standard_unstable", {})
    jvm_args_non_standard_unstable_list = build_jvm_args_non_standard_unstable_list(jvm_args_non_standard_unstable)

    jvm_args_properties = java_cfg.get("jvm_args_properties", {})
    jvm_args_properties_list = build_jvm_args_properties_list(jvm_args_properties)

    jvm_args_non_standard = " ".join(jvm_args_non_standard_list + jvm_args_non_standard_unstable_list)
    jvm_args_properties = " ".join(jvm_args_properties_list)
    jvm_args_app = " ".join(jvm_args_app_list)
    jar = java_cfg["jar"]
    cmd = f"java {jvm_args_non_standard} {jvm_args_properties} -server -cp config -jar {jar} {jvm_args_app}"
    return cmd
#
#################################################################


def generate_files_from_config(app_dir, temp_dir):
    """ tar up the files needed for config, including any embedded files;
        verifies on-disk files are present
    """
    # get all config file refs to copy them into the tarball
    files = _get_all_file_refs_from_configs(f"{app_dir}/.arryved/config")

    # for all files under the files key, create required subdirs in temp_dir
    _make_subdirs(files.keys(), temp_dir)

    for path, contents in files.items():
        file_exists_readable = os.path.exists(path) and os.path.isfile(path) and os.access(path, os.R_OK)
        temp_path = f"{temp_dir}/{path}"

        if contents:
            # when value present, dump contents to file in temp_dir
            _dump_file(contents, temp_path)
            click.echo(click.style(f"dumping file at {path}", fg="blue"))
            if file_exists_readable:
                # warn if a file is being overridden
                click.echo(click.style(f"warning: content in config is taking precedence over existing file at {path}", fg="blue"), err=True)
        else:
            # when value absent, verify that file exists on disk, and fail if it doesn't
            if not file_exists_readable:
                msg = f"expected file at {path} is missing"
                click.echo(click.style(msg, fg="red"), err=True)
                raise ValueError(msg)
            # copy the existing file to the temp_path
            click.echo(click.style(f"copying file at {path}", fg="blue"))
            _copy_file(path, temp_path)


def create_tarball(object_name, temp_dir):
    # create an archive to add all the temp_dir files, tagged to the config_spec & version/hash
    with working_directory(temp_dir):
        tarfile_path = f"{temp_dir}/{object_name}.tar.gz"
        with tarfile.open(tarfile_path, "w:gz") as tar:
            tar.add("./")
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


def copy_tree(src, dst):
    """
    recursively copy a directory tree from src to dst
    """
    try:
        shutil.copytree(src, dst)
    except OSError as e:
        # If the error was caused by the source directory being the same as the destination directory, ignore it
        if e.errno == shutil.errno.ENOTEMPTY:
            pass
        else:
            raise


@click.command()
@click.option('-e', '--environment', required=False, default=None)
@click.option('-r', '--region', required=False, default=None)
@click.option('-t', '--variant', required=False, default=None)
@click.option('-v', '--version', required=True)
@click.option('-c', '--compile', is_flag=True, required=False, default=False)
def config(environment, region, variant, version, compile):
    click.echo(click.style(f"Checking config for environment={environment},region={region},variant={variant},version={version}...", fg="green"))
    arryved_dir = "./.arryved"
    cfg_default = _load_config_file(f"{arryved_dir}/config/defaults.yaml")
    app_name = cfg_default["name"]
    git_hash = _git_hash()
    object_name = f"config-app={app_name},hash={git_hash},version={version}"

    with click_spinner.spinner():
        try:
            click.echo(click.style(f"Building config object {object_name}", fg="green"))
            config_spec = (environment, region, variant)
            merged = merge_config(arryved_dir, config_spec)

            # allow merged cfg debugging, and validation of specific configs
            # may pull this out in favor of another tool later when/if it makes sense
            if compile:
                merged_path = f"{arryved_dir}/config/config-environment={environment},region={region},variant={variant},version={version}.yaml"
                _dump_config_file(merged, merged_path)
                click.echo(click.style(f"Dumped config object {object_name} to {merged_path}", fg="blue"))
                if merged.get("java"):
                    click.echo(click.style(f"Simulating java cmd for {object_name}", fg="blue"))
                    cmd = _simulate_java_cmd(merged)
                    print(cmd)

            else:
                click.echo(click.style(f"Pushing config object {object_name} to repo", fg="blue"))
                with tempfile.TemporaryDirectory() as temp_dir:
                    generate_files_from_config(".", temp_dir)
                    copy_tree(f"{arryved_dir}", f"{temp_dir}/.arryved")
                    tarball_path = create_tarball(object_name, temp_dir)
                    push_config_artifact(storage.Client(), tarball_path)
        except Exception as e:
            click.echo(click.style(f"Server experienced an error: {e}", fg="red"), err=True)
            exit(1)
