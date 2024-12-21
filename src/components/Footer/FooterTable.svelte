<script lang="ts">
	let { header, tableData } = $props();
</script>

<div class="footerBox">
	<strong>{header}</strong>
	<table>
		<tbody>
			{#each tableData.slice(0, 3) as line}
				<tr>
					<td>{line[0]}</td>
					<td>{line[1]}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
