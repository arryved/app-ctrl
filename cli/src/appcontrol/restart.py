import click


@click.command()
@click.option("-a", "--application", required=True, help="the target application")
@click.option("-H", "--hosts", default=None, help="comma-separated list of specific hosts to restart, will restart all otherwise")
@click.option("-c", "--concurrency", default=1, help="restart concurrency as a number or percentage of instances, default=1")
def restart(application: str, hosts: str, concurrency: int):
    # hosts = "<all hosts>" if hosts is None else hosts.split(",")
    # click.echo(f"restart {application} on {hosts} with concurrency {concurrency}")
    click.echo(click.style("this command is not yet implemented", fg="yellow"))
    exit(1)
