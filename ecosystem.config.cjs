module.exports = {
	apps: [
		{
			name: 'tellulf-v9',
			script: './build/index.js',
			env_file: '.env',
			instances: 1,
			autorestart: true,
			watch: false,
			max_memory_restart: '1G'
		}
	]
};
