package cfmms

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"sync"
)

func SyncPairs(dexes []dex.Dex, client *ethclient.Client, checkpoint_path string) {

	SyncPairsWithThrottle(dexes, 100000, client, 0, checkpoint_path)
}

func SyncPairsWithThrottle(dexes []dex.Dex, step uint64, client *ethclient.Client, requests_per_second_limit uint64, checkpoint_path string) {
	//let current_block = middleware
	//.get_block_number()
	//.await
	//.map_err(CFMMError::MiddlewareError)?;
	//
	////Initialize a new request throttle
	//let request_throttle = Arc::new(Mutex::new(RequestThrottle::new(requests_per_second_limit)));
	//
	////Aggregate the populated pools from each thread
	//let mut aggregated_pools: Vec<Pool> = vec![];
	//let mut handles = vec![];
	//
	////Initialize multi progress bar
	//let multi_progress_bar = MultiProgress::new();
	//
	////For each dex supplied, get all pair created events and get reserve values
	//for dex in dexes.clone() {
	//	let middleware = middleware.clone();
	//	let request_throttle = request_throttle.clone();
	//	let progress_bar = multi_progress_bar.add(ProgressBar::new(0));
	//
	//	//Spawn a new thread to get all pools and sync data for each dex
	//	handles.push(tokio::spawn(async move {
	//		progress_bar.set_style(
	//			ProgressStyle::with_template("{msg} {bar:40.cyan/blue} {pos:>7}/{len:7}")
	//		.expect("Error when setting progress bar style")
	//		.progress_chars("##-"),
	//	);
	//
	//		//Get all of the pools from the dex
	//		progress_bar.set_message(format!("Getting all pools from: {}", dex.factory_address()));
	//
	//		let mut pools = dex
	//		.get_all_pools(
	//		request_throttle.clone(),
	//		step,
	//		progress_bar.clone(),
	//		middleware.clone(),
	//	)
	//		.await?;
	//
	//		progress_bar.reset();
	//		progress_bar.set_style(
	//		ProgressStyle::with_template("{msg} {bar:40.cyan/blue} {pos:>7}/{len:7}")
	//		.expect("Error when setting progress bar style")
	//		.progress_chars("##-"),
	//	);
	//
	//		//Get all of the pool data and sync the pool
	//		progress_bar.set_message(format!(
	//		"Getting all pool data for: {}",
	//		dex.factory_address()
	//	));
	//		progress_bar.set_length(pools.len() as u64);
	//
	//		dex.get_all_pool_data(
	//		&mut pools,
	//		request_throttle.clone(),
	//		progress_bar.clone(),
	//		middleware.clone(),
	//	)
	//		.await?;
	//
	//		//Clean empty pools
	//		pools = remove_empty_pools(pools);
	//
	//		Ok::<_, CFMMError<M>>(pools)
	//	}));
	//}
	//
	//for handle in handles {
	//	match handle.await {
	//	Ok(sync_result) => aggregated_pools.extend(sync_result?),
	//	Err(err) => {
	//{
	//	if err.is_panic() {
	//	// Resume the panic on the main task
	//	resume_unwind(err.into_panic());
	//}
	//}
	//}
	//}
	//}
	//
	////Save a checkpoint if a path is provided
	//if checkpoint_path.is_some() {
	//	let checkpoint_path = checkpoint_path.unwrap();
	//
	//checkpoint::construct_checkpoint(
	//		dexes,
	//		&aggregated_pools,
	//		current_block.as_u64(),
	//		checkpoint_path,
	//	)
	//}
	//
	////Return the populated aggregated pools vec
	//Ok(aggregated_pools)

	wg := sync.WaitGroup{}

	currentBlock, _ := client.BlockNumber(context.Background())

	request_throttle := NewThrottle(requests_per_second_limit)

	var aggregatedPools []pool.Pool

	for _, dexIns := range dexes {
		wg.Add(1)

		go func(dexIns dex.Dex) {
			defer wg.Done()

			// Get all pools from the dex
			pools, err := dexIns.GetAllPools(request_throttle, client, step)
			if err != nil {
				// Handle error
				fmt.Println(err)
				return
			}

			// Get all pool data and sync the pool
			err = dexIns.GetAllPoolsData(&pools, request_throttle, client)
			if err != nil {
				// Handle error
				fmt.Println(err)
				return
			}

			// Clean empty pools
			pools = RemoveEmptyPools(pools)

			// Append pools to aggregatedPools
			aggregatedPools = append(aggregatedPools, pools...)

		}(dexIns)
	}
	wg.Wait()

	if checkpoint_path != "" {

	//	let checkpoint_path = checkpoint_path.unwrap();
	//
	//checkpoint::construct_checkpoint(
	//		dexes,
	//		&aggregated_pools,
	//		current_block.as_u64(),
	//		checkpoint_path,
	//	)
		// Construct checkpoint
		ConstructCheckpoint(dexes, aggregatedPools, currentBlock, checkpoint_path)
	}

	}

}

func SyncPairsWithStep() {

}

func RemoveEmptyPools(pools []pool.Pool) []pool.Pool {

	var cleaned_pools []pool.Pool

	for _, p := range pools {

		switch p.(type) {
		case *pool.UniswapV2Pool:
			if p.(*pool.UniswapV2Pool).TokenA != utils.Address0 {
				cleaned_pools = append(cleaned_pools, p)
			}
		case *pool.UniswapV3Pool:
			if p.(*pool.UniswapV3Pool).TokenA != utils.Address0 {
				cleaned_pools = append(cleaned_pools, p)
			}
		}

	}

	return cleaned_pools
}
