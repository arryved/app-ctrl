import atexit
import certifi
import click
import os
import platform
import subprocess
import tempfile

from appcontrol.restart import restart
from appcontrol.status import status
from appcontrol.config import config
from appcontrol.deploy import deploy
from appcontrol.version import version


def cleanup():
    print("Cleaning up...")
    temp_dir = os.environ.get("APP_CONTROL_TEMP_DIR", "")
    if os.path.exists(temp_dir):
        for root, dirs, files in os.walk(temp_dir, topdown=False):
            for name in files:
                os.remove(os.path.join(root, name))
            for name in dirs:
                os.rmdir(os.path.join(root, name))
        os.rmdir(temp_dir)


def merge_ca():
    temp_dir = os.environ.get("APP_CONTROL_TEMP_DIR", "")
    system_ca_path = certifi.where()
    merged_ca_path = f"{temp_dir}/ca.pem"
    with open(system_ca_path, 'r') as f:
        contents = f.read()
        with open(merged_ca_path, 'w') as ca_file:
            ca_file.write(contents)
    if platform.system() == "Darwin":
        command = "security find-certificate -c Arryved -p"
        output = subprocess.check_output(command, shell=True)
        with open(merged_ca_path, 'a') as ca_file:
            ca_file.write("\n" + output.decode("utf-8"))
    return merged_ca_path


@click.group()
def cli():
    atexit.register(cleanup)
    with tempfile.TemporaryDirectory(delete=False) as temp_dir:
        os.environ["APP_CONTROL_TEMP_DIR"] = temp_dir
        os.environ["REQUESTS_CA_BUNDLE"] = merge_ca()


cli.add_command(status)
cli.add_command(restart)
cli.add_command(config)
cli.add_command(deploy)
cli.add_command(version)

if __name__ == "__main__":
    cli()
