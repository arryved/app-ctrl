import click
import click_spinner
import json
import math
import requests
import warnings

from appcontrol.common import constants


warnings.filterwarnings("ignore")


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-a', '--application', required=True, default="")
@click.option('-r', '--region', required=False, default="us-central1")
@click.option('-t', '--variant', required=False, default="default")
def deploy(environment, application, variant):
    action = "deploy"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}/{application}/{variant}")
    click.echo(click.style(f"Connecting to {url} ...", fg="green"))

    with click_spinner.spinner():
        # TODO - use CA cert
        response = requests.get(url, verify=False)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        decoded = json.loads(response.text)
        click.echo(click.style(f"Server experienced an error: {decoded}", fg="red"), err=True)
        exit()

    result = json.loads(response.text)
    print(result)
