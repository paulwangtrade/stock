package data

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/logger"

	"github.com/chromedp/chromedp"
)


// BrowserPool 浏览器池
type BrowserPool struct {

	pool chan context.Context

	size int
}



// NewBrowserPool 创建浏览器池
func NewBrowserPool(size int) *BrowserPool {


	if size <= 0 {
		size = 1
	}


	pool := make(chan context.Context,size)



	for i:=0;i<size;i++{


		ctx:=createBrowserContext()


		pool <- ctx

	}



	return &BrowserPool{

		pool:pool,

		size:size,
	}

}



// 创建浏览器 context
func createBrowserContext() context.Context {


	path:=GetSettingConfig().BrowserPath



	options:=[]chromedp.ExecAllocatorOption{


		// 调试阶段关闭无头
		// 测试成功以后改 true
		chromedp.Flag(
			"headless",
			false,
		),


		chromedp.Flag(
			"no-sandbox",
			true,
		),


		chromedp.Flag(
			"disable-setuid-sandbox",
			true,
		),


		chromedp.Flag(
			"disable-gpu",
			true,
		),


		chromedp.Flag(
			"disable-dev-shm-usage",
			true,
		),


		chromedp.Flag(
			"disable-extensions",
			true,
		),


		chromedp.Flag(
			"disable-popup-blocking",
			true,
		),


		chromedp.Flag(
			"disable-background-networking",
			true,
		),


		chromedp.Flag(
			"disable-sync",
			true,
		),


		chromedp.Flag(
			"ignore-certificate-errors",
			true,
		),



		chromedp.Flag(
			"disable-features",
			"IsolateOrigins,site-per-process",
		),



		chromedp.UserAgent(
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/133 Safari/537.36",
		),


	}



	if path!=""{


		options=append(
			options,
			chromedp.ExecPath(path),
		)

	}




	// 浏览器生命周期
	allocCtx,_:=chromedp.NewExecAllocator(
		context.Background(),
		options...,
	)



	browserCtx,_:=chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(
			logger.SugaredLogger.Infof,
		),
	)



	return browserCtx

}




// 获取浏览器
func(pool *BrowserPool)Get()context.Context{


	return <-pool.pool

}





// 放回浏览器
func(pool *BrowserPool)Put(ctx context.Context){



	select{


	case pool.pool<-ctx:


	default:

		chromedp.Cancel(ctx)

	}


}




// 关闭
func(pool *BrowserPool)Close(){



	close(pool.pool)



	for ctx:=range pool.pool{


		chromedp.Cancel(ctx)

	}

}





// FetchPage 获取网页
func(pool *BrowserPool)FetchPage(
	url string,
	waitVisible string,
)(string,error){



	ctx:=pool.Get()



	// 默认放回
	defer func(){

		if ctx.Err()!=nil{

			logger.SugaredLogger.Warn(
				"browser context dead recreate",
			)

			pool.pool<-createBrowserContext()


		}else{


			pool.Put(ctx)

		}


	}()



	var htmlContent string



	logger.SugaredLogger.Infof(
		"navigate start url=%s",
		url,
	)




	// 单次访问 timeout
	runCtx,cancel:=context.WithTimeout(
		ctx,
		60*time.Second,
	)


	defer cancel()




	err := chromedp.Run(
		runCtx,
	
		chromedp.Navigate(url),
	
		chromedp.Sleep(
			2*time.Second,
		),
	
		chromedp.Evaluate(
			`document.documentElement.outerHTML`,
			&htmlContent,
		),
	)



	if err!=nil{


		logger.SugaredLogger.Errorf(
			"chromedp failed url=%s err=%v",
			url,
			err,
		)


		return "",err

	}




	if htmlContent==""{


		return "",
			fmt.Errorf(
				"empty html",
			)

	}




	logger.SugaredLogger.Infof(
		"crawler success url=%s size=%d",
		url,
		len(htmlContent),
	)



	return htmlContent,nil

}