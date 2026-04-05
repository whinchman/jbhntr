/* sankey.js — D3 Sankey diagram renderer for the Stats page */
document.addEventListener('DOMContentLoaded', function () {
  var dataEl = document.getElementById('sankey-data');
  if (!dataEl) return;

  var data;
  try {
    data = JSON.parse(dataEl.textContent);
  } catch (e) {
    return;
  }

  var links = data.links;
  if (!links || links.length === 0) return;

  // Colour map: status name → CSS variable
  var colorMap = {
    'Discovered':   '--color-accent',
    'Approved':     '--color-success',
    'Rejected':     '--color-danger',
    'Applied':      '--color-info',
    'Interviewing': '--color-purple',
    'Won':          '--color-teal',
    'Lost':         '--color-warning',
    'Pending':      '--color-text-muted',
  };

  function nodeColor(name) {
    var root = document.documentElement;
    var cssVar = colorMap[name] || '--color-accent';
    return getComputedStyle(root).getPropertyValue(cssVar).trim() || '#4f46e5';
  }

  // Derive unique nodes from links
  var nameSet = new Set();
  links.forEach(function (l) {
    nameSet.add(l.source);
    nameSet.add(l.target);
  });
  var nodeNames = Array.from(nameSet);
  var nodeIndex = {};
  nodeNames.forEach(function (n, i) { nodeIndex[n] = i; });

  var nodes = nodeNames.map(function (n) { return { name: n }; });
  var sankeyLinks = links.map(function (l) {
    return {
      source: nodeIndex[l.source],
      target: nodeIndex[l.target],
      value:  l.value,
    };
  });

  // Size
  var container = document.getElementById('sankey-diagram');
  var w = container.clientWidth || 600;
  var h = Math.max(200, nodes.length * 40);

  var svg = d3.select('#sankey-diagram')
    .append('svg')
    .attr('width', w)
    .attr('height', h)
    .attr('viewBox', '0 0 ' + w + ' ' + h);

  // Build sankey layout
  var sankey = d3.sankey()
    .nodeWidth(20)
    .nodePadding(12)
    .extent([[1, 1], [w - 1, h - 1]]);

  var graph = sankey({
    nodes: nodes.map(function (d) { return Object.assign({}, d); }),
    links: sankeyLinks.map(function (d) { return Object.assign({}, d); }),
  });

  // Draw links
  svg.append('g')
    .attr('fill', 'none')
    .selectAll('path')
    .data(graph.links)
    .join('path')
    .attr('d', d3.sankeyLinkHorizontal())
    .attr('stroke', function (d) { return nodeColor(d.source.name); })
    .attr('stroke-width', function (d) { return Math.max(1, d.width); })
    .attr('stroke-opacity', 0.35)
    .append('title')
    .text(function (d) {
      return d.source.name + ' \u2192 ' + d.target.name + ': ' + d.value;
    });

  // Draw nodes
  var nodeGroup = svg.append('g')
    .selectAll('g')
    .data(graph.nodes)
    .join('g');

  nodeGroup.append('rect')
    .attr('x', function (d) { return d.x0; })
    .attr('y', function (d) { return d.y0; })
    .attr('height', function (d) { return d.y1 - d.y0; })
    .attr('width', function (d) { return d.x1 - d.x0; })
    .attr('fill', function (d) { return nodeColor(d.name); })
    .append('title')
    .text(function (d) { return d.name + ': ' + d.value; });

  // Draw labels
  nodeGroup.append('text')
    .attr('x', function (d) { return d.x0 < w / 2 ? d.x1 + 6 : d.x0 - 6; })
    .attr('y', function (d) { return (d.y1 + d.y0) / 2; })
    .attr('dy', '0.35em')
    .attr('text-anchor', function (d) { return d.x0 < w / 2 ? 'start' : 'end'; })
    .attr('font-size', '12px')
    .attr('fill', 'var(--color-text)')
    .text(function (d) { return d.name; });
});
