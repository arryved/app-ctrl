import click
from importlib.metadata import version as metadata_version

@click.command('version', short_help='print the CLI version')
def version():
    click.echo(metadata_version(click.get_current_context().parent.info_name))
