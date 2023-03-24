import click, click_spinner
import json
import math
import requests
import warnings

from ansitable import ANSITable, Column
from sys import stderr

warnings.filterwarnings("ignore")

api_hosts_by_env = {
    "dev": "app-control.dev.arryved.com",
    "prod": "app-control.prod.arryved.com",
    "sandbox": "app-control.dev.arryved.com",
    "staging": "app-control.prod.arryved.com",
}

@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-a', '--application', required=True)
def status(environment, application):
    port = 1026
    action = "status"
    api_host = api_hosts_by_env[environment]
    url = (f"https://{api_host}:{port}/{action}/{environment}/{application}")
    click.echo(click.style(f"Connecting to {url}...", fg="green"))

    with click_spinner.spinner():
        # TODO - use CA cert
        response = requests.get(url, verify=False)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        decoded = json.loads(response.text)
        click.echo(click.style(f"Server experienced an error: {decoded}", fg="red"), err=True)
        exit()

    table = ANSITable(
        Column("Host", headstyle="bold"),
        Column("Installed", headstyle="bold"),
        Column("Running", headstyle="bold"),
        Column("Config", headstyle="bold"), 
        Column("Health|Port", headstyle="bold"),
         border="thin"
    )

    result = json.loads(response.text)
    for host, status in result.items():
        installed = "<<red>>?"
        running = "<<red>>?"
        config = "<<red>>?"
        healths = "?"
        if status:
            installed = render_version(status["versions"]["installed"])
            running = render_version(status["versions"]["running"])
            config = status["versions"]["config"]
            config = config if config else "n/a"
            healths = []
            for h in status["health"]:
                port = h["port"]
                health = "OK" if h["healthy"] else "DOWN"
                health = "?" if h["unknown"] else health
                healths.append(f"{health}|{port}")    
        healths = " ".join(healths)
        color = "red" if "?" in healths or "DOWN" in healths else "green"
        healths = f"<<{color}>>{healths}" if healths else "n/a"
        table.row(host, installed, running, config, healths)
    table.print()

def render_version(version):
    major = version["major"]
    minor = version["minor"]
    patch = version["patch"]
    build = version["build"]

    major = "" if major < 0 else str(major)
    minor = "" if minor < 0 else str(minor)
    patch = "" if patch < 0 else str(patch)
    build = "" if build < 0 else str(build)

    full = ".".join([e for e in [major, minor, patch, build] if e])
    if full == "":
        full = "n/a"

    return full

