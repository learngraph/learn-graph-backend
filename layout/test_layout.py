import json
import subprocess


def layout(input):
    data = json.dumps(input)
    print("inserting data:", data)
    completed_process = subprocess.run(
        ["go", "run", "../cmd/gen-layout/main.go"],
        input=data,
        text=True,
        stdout=subprocess.PIPE,
    )
    return json.loads(completed_process.stdout)


def test_layout_json_io():
    input = {"nodes": [{"name": "A"}, {"name": "B"}]}
    output = layout(input)
    assert "nodes" in output
    nodes = output["nodes"]
    assert len(nodes) == 2
    assert nodes[0]["pos"] == [1, 1]
    assert nodes[1]["pos"] == [1, 1]
