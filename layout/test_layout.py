import json
import random
import subprocess
import numpy as np
import matplotlib.pyplot as plt


def layout(input):
    data = json.dumps(input)
    completed_process = subprocess.run(
        ["go", "run", "../cmd/gen-layout/main.go"],
        input=data,
        text=True,
        stdout=subprocess.PIPE,
    )
    return json.loads(completed_process.stdout)


def generate_graph(n_nodes=1000, n_edges=1000):
    def random_node(except_node=None):
        n = random.randint(0, n_nodes)
        if n == except_node:
            n += 1
        if n == n_nodes:
            n -= 2
        return n

    nodes = []
    for i in range(n_nodes):
        nodes.append({})
    edges = []
    for i in range(n_edges):
        src = random_node()
        edges.append({"source": src, "target": random_node(src)})
    return {"nodes": nodes, "edges": edges}


def to_numpy(graph):
    return np.array([node["pos"] for node in graph["nodes"]]), np.array(
        [[edge["source"], edge["target"]] for edge in graph["edges"]]
    )


def _plot(graph, plt):
    nodes, edges = to_numpy(graph)
    plt.scatter(nodes[:, 0], nodes[:, 1], marker=".")
    # optimize?
    # plt.plot(nodes[edges[:, 0], 0], nodes[edges[:, 0], 1], color='red')
    # plt.plot(nodes[edges[:, 1], 0], nodes[edges[:, 1], 1], color='red')
    for edge in edges:
        start = edge[0]
        end = edge[1]
        plt.plot(
            [nodes[start][0], nodes[end][0]],
            [nodes[start][1], nodes[end][1]],
            color="red",
        )


def plot(graph, file):
    _plot(graph, plt)
    plt.savefig(file)
    plt.clf()


def plot_series(graph, max=10):
    o = layout(graph)
    plot(o, f"o0.png")
    for i in range(1, max):
        o = layout(o)
        plot(o, f"o{i}.png")


def test_layout_json_io():
    input = {"nodes": [{"name": "A"}, {"name": "B"}]}
    output = layout(input)
    assert "nodes" in output
    nodes = output["nodes"]
    assert len(nodes) == 2
    assert nodes[0]["pos"] == [1, 1]
    assert nodes[1]["pos"] == [1, 1]
