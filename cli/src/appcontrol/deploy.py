import click
import click_spinner
import json
import math
import requests
import warnings

from appcontrol.common import constants
from appcontrol.auth import token


warnings.filterwarnings("ignore")


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-a', '--application', required=True)
@click.option('-r', '--region', required=False, default="central")
@click.option('-t', '--variant', required=False, default="default")
@click.option('-n', '--version', required=True)
def deploy(environment, application, region, variant, version):
    action = "deploy"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}/{application}/{region}/{variant}")
    click.echo(click.style(f"Connecting to {url} ...", fg="green"))

    with click_spinner.spinner():
        body = {
                # "principal": "",  # deprecated, this can/should be pulled server-side from the validated token
                "concurrency": "1",
                "version": version,
        }
        # TODO - use CA cert
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.post(url, json=body, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        decoded = json.loads(response.text)
        click.echo(click.style(f"Server experienced an error: {decoded}", fg="red"), err=True)
        exit()

    result = json.loads(response.text)
    print(result)
