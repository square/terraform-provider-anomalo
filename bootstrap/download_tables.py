import argparse
import os
import json
import logging

from python_terraform import Terraform
import anomalo

logger = logging.getLogger(__name__)

color_code_regex = re.compile(r'\x1B\[\d+(;\d+){0,2}m')


def get_anomalo_client(secret_file, host, api_token):
    if host is not None and api_token is not None:
        pass
    elif secret_file is not None:
        with open(secret_file, 'r') as f:
            secrets = json.load(f)
            return anomalo.Client(**secrets)
    else:
        host = os.environ.get("ANOMALO_INSTANCE_HOST")
        api_token = os.environ.get("ANOMALO_API_SECRET_TOKEN")

    if (host is None or host == "") and (api_token is None or api_token == ""):
        raise Exception(f"Invalid anomalo secrets. Got host {host} and token {api_token}")

    client = anomalo.Client(host=host, api_token=api_token)
    pong = client.ping()
    if "ping" not in pong or pong['ping'] != "pong":
        raise Exception(f"Did not get a valid anomalo response from /ping. Got: {pong}")

    print("Successfully connected to Anomalo")
    return client

def tables_from_file(file_path):
    with open(file_path, 'r') as f:
        tables = [line.strip() for line in f]
    print(f"Loaded {len(tables)} tables from {file_path}")
    return tables


def run(args):
    terraform_client = Terraform()
    anomalo_client = get_anomalo_client(args.anomalo_secret_file, args.anomalo_host, args.anomalo_token)

    tables_to_fetch = tables_from_file(args.table_file)

    failed_tables = []
    for table_name in tables_to_fetch:
        print(f"Downloading table {table_name}")

        # Fetch it from anomalo to make sure it exists & to get the table_id
        table = anomalo_client.get_table_information(table_name=table_name)
        if "id" not in table:
            failed_tables.append((table_name, f"Unable to fetch table {table_name} from anomalo. Skipping."))
            logger.error(failed_tables[-1][-1])
            continue
        else:
            print(f"Fetched table {table_name} from anomalo")

        table_full_name = f'{table["warehouse"]["name"]}.{table["full_name"]}'
        table_terraform_resource_name = table_full_name.replace(".", "__")
        table_terraform_state_reference = f"anomalo_table.{table_terraform_resource_name}"

        # Insert the table config boilerplate
        boilerplate_file_content = [f'resource "anomalo_table" "{table_terraform_resource_name}" {{}}']\

        # Fetch checks for the table & insert boilerplate
        checks_response = anomalo_client.get_checks_for_table(table['id'])
        if 'checks' not in checks_response:
            failed_tables.append((table_name, f"Unable to fetch checks for table {table_name} from anomalo. Skipping."))
            logger.error(failed_tables[-1][-1])
            continue

        all_checks = checks_response['checks']
        user_checks = [check for check in all_checks if check['check_id'] > 0]
        print(f"Fetched {len(user_checks)} checks for table {table_name}")

        tf_anomalo_check_id_mapping = {}
        for check in user_checks:
            terraform_name = f'{check["check_type"]}-{check["check_static_id"]}'
            boilerplate_file_content.append(f'resource "anomalo_check" "{terraform_name}" {{}}')
            tf_anomalo_check_id_mapping[f'anomalo_check.{terraform_name}'] = f'{table["id"]},{check["check_static_id"]}'

        # Write file
        out_path = f'{args.out_dir}/{table_terraform_resource_name}.tf'
        if os.path.exists(out_path):
            failed_tables.append((table_name, f"File {out_path} already exists. Skipping for table {table_name}."))
            logger.error(failed_tables[-1][-1])
            continue
        else:
            with open(out_path, 'w') as f:
                f.write("\n".join(boilerplate_file_content))

        final_file_output = []

        # Run terraform imports
        try:
            print(run_terraform(terraform_client, "import", table_terraform_state_reference, table_full_name))
            imported_table_state = (run_terraform(terraform_client, "state", "show", table_terraform_state_reference))
            print(imported_table_state)
            final_file_output.append(imported_table_state)

            for state_reference, import_id in tf_anomalo_check_id_mapping.items():
                print(run_terraform(terraform_client, "import", state_reference, str(import_id)))

                imported_check_state = (run_terraform(terraform_client, "state", "show", state_reference))
                print(imported_check_state)
                final_file_output.append(imported_check_state)
        except Exception as e:
            logger.error(e)
            failed_tables.append((table_name, f"Unable to execute terraform commands for table {table_name}. Skipping"))
            logger.error(failed_tables[-1][-1])
            continue

        out_path = f'{args.out_dir}/{table_terraform_resource_name}.tf'
        with open(out_path, 'w') as f:
            # Some terraform commands provide a -no-color flag that strips the color codes for us. `terraform show` does
            # this but does not allow specifying specific resources. `terraform state show` does not support this.
            f.write(color_code_regex.sub('', "\n".join(final_file_output)))

    if len(failed_tables) > 0:
        logger.error(f"Failed import the following tables: {failed_tables}")
    else
        logger.info("Successfully imported all tables.")


def run_terraform(terraform_client, *args):
    return_code, stdout, stderr = terraform_client.cmd(*args)
    # if return_code != 0:
    #     logger.error(stdout)
    logger.error(stderr)
    #     raise Exception(f"Error running terraform command with args {args}. Got return code {return_code}.")
    return stdout


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("--table-file", required=True)
    parser.add_argument("--out-dir", required=False, default="./")
    parser.add_argument("--anomalo-secret-file", required=False)
    parser.add_argument("--anomalo-host", required=False)
    parser.add_argument("--anomalo-token", required=False)
    args, _ = parser.parse_known_args()

    run(args)
