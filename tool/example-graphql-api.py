"""
Python example to use our GraphQL API.

Alternatively you can directly curl to experiment:
```sh
# set the language
lang="en" # or "de", "zh", ...
curl -d '{"query":"query nodeCompletion($substring: String!) {\n nodeCompletion(substring: $substring) {\n id\n description\n resources\n position {\n x\n y\n z\n }\n }\n }", "variables": {"substring": "math'"'"'"}}' -H 'Content-Type: application/json' -H "Language: $lang" -X POST 'https://prototype.learngraph.org/graphql'
```
"""
import requests

# Define the GraphQL query without the position field
query = """
query NodeCompletion($substring: String!) {
  nodeCompletion(substring: $substring) {
    id
    description
    resources
  }
}
"""


def queryNodes(url, keyword, language="en"):
    """
    Queries the GraphQL endpoint for nodes matching the given keyword.

    Args:
        keyword (str): The substring to search for in node descriptions.
        language (str): The language in which you want to query nodes, one of {de, en, zh, es}

    Returns:
        list: A list of nodes matching the keyword.
    """
    variables = {"substring": keyword}
    headers = {
        "Content-Type": "application/json",
        "Language": language,
    }
    try:
        response = requests.post(
            url, json={"query": query, "variables": variables}, headers=headers
        )
        response.raise_for_status()
        json_data = response.json()
        # Check for errors in the graphql response
        if "errors" in json_data:
            print("Errors:", json_data["errors"])
            return []
        else:
            # Extract the nodes from the response
            nodes = json_data["data"]["nodeCompletion"]
            return nodes
    except requests.exceptions.RequestException as e:
        print(f"An error occurred: {e}")
        return []


if __name__ == "__main__":
    url = "https://learngraph.org/graphql"
    keyword = "mathem"
    nodes = queryNodes(url, keyword)
    for node in nodes:
        print(f"Node ID: {node['id']}")
        print(f"Description: {node['description']}")
        print(f"Resources: {node.get('resources', 'None')}")
        print("---")
