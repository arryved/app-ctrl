import click
import click_spinner
import json
import math
import requests
import warnings

from ansitable import ANSITable, Column

from appcontrol.common import constants


warnings.filterwarnings("ignore")


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-a', '--application', required=False, default=[""], multiple=True)
@click.option('--canary', 'canary', required=False, flag_value='canary', help="Show only canary hosts")
@click.option('--no-canary', 'canary', required=False, flag_value='no-canary', help="Do not show canary hosts")

def status(environment, application, canary):
    action = "status"
    api_host = constants["api_hosts_by_env"][environment]
    for app in application:
        url = (f"{api_host}/{action}/{environment}/{app}")
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
        print_status_table(app, result, canary)


def print_status_table(application, result, canary):
    table = ANSITable(
        Column("Application", headstyle="bold"),
        Column("Host", headstyle="bold"),
        Column("Meta", headstyle="bold"),
        Column("Installed", headstyle="bold"),
        Column("Running", headstyle="bold"),
        Column("Config", headstyle="bold"),
        Column("Health|Port", headstyle="bold"),
        border="thin"
    )

    if application == "":
        results = result
    else:
        results = {application: result}

    for application, result in results.items():
        for host, status in result["hostStatuses"].items():
            is_canary = "canary" if host in result["attributes"]["canaries"] else ""
            meta = ','.join([is_canary])
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
            if canary == "canary" and not is_canary:
                pass
            elif canary == "no-canary" and is_canary:
                pass
            else:
                table.row(application, host, meta, installed, running, config, healths)

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
