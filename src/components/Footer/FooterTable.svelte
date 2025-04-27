<script lang="ts">
	interface Props {
		header: string;
		tableData: string[][];
	}

	export let header: Props['header'] = 'default name';
	export let tableData: Props['tableData'] = [];
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
